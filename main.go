package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/urfave/cli/v3"
)

func main() {
	var file string

	cmd := &cli.Command{
		Name:  "dep-tree",
		Usage: "parse a dependency tree file (e.g. gradle dependencies / mvn dependency:tree output)",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "file",
				Aliases:     []string{"f"},
				Usage:       "path to the dependency tree file",
				Value:       "tree.txt",
				Destination: &file,
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			data, err := os.ReadFile(file)
			if err != nil {
				return fmt.Errorf("read %s: %w", file, err)
			}

			dependencies := parseDependencies(string(data))
			for _, dep := range dependencies {
				fmt.Println(dep)
			}
			return nil
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
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
