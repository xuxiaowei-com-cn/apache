package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/urfave/cli/v3"
	"golang.org/x/net/html/charset"
)

func main() {
	var file string
	var licenseFile string
	var excludeGroups []string
	var checkVersion bool

	cmd := &cli.Command{
		Name:  "license-check",
		Usage: "parse a dependency tree file (e.g. mvn dependency:tree output) and check against a license file",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "file",
				Aliases:     []string{"f"},
				Usage:       "path to the dependency tree output (e.g. mvn dependency:tree)",
				Value:       "tree.txt",
				Destination: &file,
			},
			&cli.StringFlag{
				Name:        "license-file",
				Aliases:     []string{"l"},
				Usage:       "path to the license whitelist file",
				Value:       "LICENSE",
				Destination: &licenseFile,
			},
			&cli.StringSliceFlag{
				Name:        "exclude-group",
				Aliases:     []string{"e"},
				Usage:       "exclude dependencies matching this Maven groupId (repeatable)",
				Destination: &excludeGroups,
			},
			&cli.BoolFlag{
				Name:        "check-version",
				Aliases:     []string{"v"},
				Usage:       "include version in license key matching (default: true)",
				Value:       true,
				Destination: &checkVersion,
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			data, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("read %s: %w", file, err)
			}

			licenseData, err := os.ReadFile(licenseFile)
			if err != nil {
				return fmt.Errorf("read %s: %w", licenseFile, err)
			}
			licenseContent := string(licenseData)

			localRepo, err := mvnLocalRepo()
			if err != nil {
				return fmt.Errorf("get maven local repository: %w", err)
			}

			// Print project name
			parseProjectName(string(data))

			var unmatched []string
			seen := make(map[string]bool)
			dependencies := parseDependencies(string(data))
			for _, dep := range dependencies {
				// Parse full Maven coordinate (for future use)
				mc := parseCoordinate(dep, localRepo)

				// Deduplicate by GroupId, ArtifactId, and Version — the same
				// dependency may appear multiple times in the tree (e.g. as a
				// transitive dependency of different parents).
				dedupKey := mc.GroupId + ":" + mc.ArtifactId + ":" + mc.Version
				if seen[dedupKey] {
					continue
				}
				seen[dedupKey] = true

				// Skip excluded groupIds
				if isExcluded(mc.GroupId, excludeGroups) {
					continue
				}

				key := mc.GroupId + ":" + mc.ArtifactId + " "
				if checkVersion {
					key += mc.Version + " "
				}
				if !strings.Contains(licenseContent, key) {
					unmatched = append(unmatched, dep)
					fmt.Println("NOT FOUND in", licenseFile+":", dep, mc.Licenses)
				}
			}

			if len(unmatched) > 0 {
				return fmt.Errorf("%d dependencies not found in %s", len(unmatched), licenseFile)
			}

			fmt.Println("All dependencies found in", licenseFile)
			return nil
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

// isExcluded reports whether groupId matches any entry in the exclude list.
func isExcluded(groupId string, excludeGroups []string) bool {
	return slices.Contains(excludeGroups, groupId)
}

// Pom holds the licenses section of a Maven POM for XML parsing.
type Pom struct {
	XMLName    xml.Name     `xml:"project"`
	Parent     *PomParent   `xml:"parent"`
	GroupId    string       `xml:"groupId"`
	ArtifactId string       `xml:"artifactId"`
	Version    string       `xml:"version"`
	Licenses   []PomLicense `xml:"licenses>license"`
}

type PomParent struct {
	GroupId    string `xml:"groupId"`
	ArtifactId string `xml:"artifactId"`
	Version    string `xml:"version"`
}

type PomLicense struct {
	Name         string `xml:"name"`
	Url          string `xml:"url"`
	Distribution string `xml:"distribution"`
	Comments     string `xml:"comments"`
}

// License holds the license information of a Maven artifact.
type License struct {
	Name         string
	Url          string
	Distribution string
	Comments     string
}

// MavenCoordinate holds the parsed components of a Maven coordinate string
// in the form "groupId:artifactId:type[:classifier]:version[:scope]".
type MavenCoordinate struct {
	Text       string // original raw coordinate string (e.g. "org.springframework.boot:spring-boot-starter-security:jar:3.5.2:compile")
	Parent     *PomParent
	GroupId    string
	ArtifactId string
	Type       string
	Classifier string
	Version    string
	Scope      string
	Licenses   []License
}

// RepoPath returns the relative path of this coordinate within a Maven
// local repository (e.g. "com/example/my-lib/1.0.0/my-lib-1.0.0.jar").
func (m MavenCoordinate) RepoPath() string {
	groupPath := filepath.Join(strings.Split(m.GroupId, ".")...)
	filename := m.ArtifactId + "-" + m.Version
	if m.Classifier != "" {
		filename += "-" + m.Classifier
	}
	if m.Type != "" && m.Type != "jar" {
		filename += "." + m.Type
	} else {
		filename += ".jar"
	}
	return filepath.Join(groupPath, m.ArtifactId, m.Version, filename)
}

// PomPath returns the relative path of the POM for this coordinate
// within a Maven local repository (e.g. "org/springframework/boot/spring-boot-starter-security/3.5.2/spring-boot-starter-security-3.5.2.pom").
func (m MavenCoordinate) PomPath() string {
	groupPath := filepath.Join(strings.Split(m.GroupId, ".")...)
	filename := m.ArtifactId + "-" + m.Version + ".pom"
	return filepath.Join(groupPath, m.ArtifactId, m.Version, filename)
}

// mvnLocalRepo returns the Maven local repository path by running
// mvn help:evaluate -Dexpression=settings.localRepository -q -DforceStdout.
func mvnLocalRepo() (string, error) {
	out, err := exec.Command("mvn", "help:evaluate",
		"-Dexpression=settings.localRepository",
		"-q",
		"-DforceStdout",
	).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// parseCoordinate parses a Maven coordinate string and returns a MavenCoordinate.
// The input format is "groupId:artifactId:type[:classifier]:version[:scope]".
// localRepo is the path to the Maven local repository, used to read the POM for licenses.
func parseCoordinate(coord, localRepo string) MavenCoordinate {
	parts := strings.Split(coord, ":")
	if len(parts) < 3 {
		panic(coord)
	}
	partsLen := len(parts)
	version := parts[partsLen-1]
	// Check if the last segment looks like a scope rather than a version.
	// Versions usually contain digits or dots; scopes are plain words.
	if isScope(version) {
		version = parts[partsLen-2]
	}

	result := MavenCoordinate{
		Text:    coord,
		Type:    parts[2],
		Version: version,
	}

	// Assign groupId and artifactId
	result.GroupId = parts[0]
	result.ArtifactId = parts[1]

	// Remaining parts between packaging and version are classifier
	// And between version and end is scope
	// Normalized layout: [0]=groupId [1]=artifactId [2]=type ... version ... scope?
	versionIdx := indexOf(parts, version)

	// Everything between type (idx 2) and version is classifier
	if versionIdx > 3 {
		result.Classifier = strings.Join(parts[3:versionIdx], "-")
	}
	// Everything after version is scope
	if versionIdx < partsLen-1 {
		result.Scope = parts[versionIdx+1]
	}

	// Update version position from version
	result.Version = version

	// Read licenses from POM file in local repository
	pomPath := filepath.Join(localRepo, result.PomPath())
	if data, err := os.ReadFile(pomPath); err == nil {
		var pom Pom
		decoder := xml.NewDecoder(bytes.NewReader(data))
		decoder.CharsetReader = charset.NewReaderLabel
		pomXml := decoder.Decode(&pom)
		if pomXml == nil {
			for _, l := range pom.Licenses {
				result.Licenses = append(result.Licenses, License{
					Name:         l.Name,
					Url:          l.Url,
					Distribution: l.Distribution,
					Comments:     l.Comments,
				})
			}

			if pom.Parent != nil {
				result.Parent = &PomParent{
					GroupId:    pom.Parent.GroupId,
					ArtifactId: pom.Parent.ArtifactId,
					Version:    pom.Parent.Version,
				}
			}

			if result.Licenses == nil {
				if strings.Contains(string(data), "https://www.apache.org/licenses/LICENSE-2.0") ||
					strings.Contains(string(data), "http://www.apache.org/licenses/LICENSE-2.0") {
					result.Licenses = append(result.Licenses, License{
						Name:         "Apache-2.0",
						Url:          "https://www.apache.org/licenses/LICENSE-2.0.txt",
						Distribution: "",
						Comments:     "",
					})
				}
			}

			if result.Licenses == nil && pom.Parent != nil {

				coordTmp := pom.Parent.GroupId + ":" + pom.Parent.ArtifactId + ":pom:" + pom.Parent.Version + ":import"
				mavenCoordinateTmp := parseCoordinateRecursion(coordTmp, localRepo)

				if mavenCoordinateTmp.Licenses != nil {
					result.Licenses = mavenCoordinateTmp.Licenses
				}
			}
		}
	}

	return result
}

func parseCoordinateRecursion(coord, localRepo string) MavenCoordinate {
	pom := parseCoordinate(coord, localRepo)
	if pom.Licenses != nil {
		return pom
	}
	if pom.Parent != nil {
		coordTmp := pom.Parent.GroupId + ":" + pom.Parent.ArtifactId + ":pom:" + pom.Parent.Version + ":import"
		return parseCoordinateRecursion(coordTmp, localRepo)
	}

	return pom
}

// isScope reports whether s looks like a Maven scope rather than a version.
func isScope(s string) bool {
	switch s {
	case "compile", "runtime", "test", "system", "provided", "import":
		return true
	}
	return false
}

// indexOf returns the first index of s in ss, or -1 if not found.
func indexOf(ss []string, s string) int {
	for i, v := range ss {
		if v == s {
			return i
		}
	}
	return -1
}

// parseProjectName prints the Maven plugin output line (e.g. "--- maven-dependency-plugin:3.6.1:tree @ project-name ---")
// and the following line (the project coordinate) from the dependency tree output.
func parseProjectName(input string) {
	scanner := bufio.NewScanner(strings.NewReader(input))
	hit := -1
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if hit+1 == lineNum {
			fmt.Printf("%s\n", line)
			hit = -1
		} else if strings.Contains(line, "dependency:") && strings.Contains(line, ":tree") && strings.Contains(line, " @ ") {
			fmt.Printf("%s\n", line)
			hit = lineNum
		}
	}
}

// parseDependencies extracts dependency names from a tree-format file.
// It scans each line for "+- " or "\- " prefixes and returns the
// content after them, including any scope suffix (e.g. :compile, :runtime, :test, :system, :provided).
func parseDependencies(input string) []string {
	var dependencies []string

	scanner := bufio.NewScanner(strings.NewReader(input))
	for scanner.Scan() {
		line := scanner.Text()

		// Extract content after "+- "
		if _, after, ok := strings.Cut(line, "+- "); ok {
			dependencies = append(dependencies, after)
		}
		// Extract content after "\- "
		if _, after, ok := strings.Cut(line, "\\- "); ok {
			dependencies = append(dependencies, after)
		}
	}

	return dependencies
}
