# Apache 阿帕奇

[English](README.md) | [中文](README-zh.md)

该项目仅用于辅助处理本人在 Apache 基金会的工作

## CI

### [incubator-seata.yml](.github/workflows/incubator-seata.yml)

> 用于检查 [incubator-seata](https://github.com/apache/incubator-seata) 发布前的投票工作

> 由于个人网络下载 https://dist.apache.org/repos/dist/ 文件速度较慢，但可以正常访问 GitHub 页面。

> 为了每次投票及相关内容的减少重复性工作，于是增加了此 CI

#### download

下载发布工件（二进制包、源码包、签名、校验和、KEYS），上传为共享工件供后续 job 使用

#### gpg

导入 KEYS，验证二进制包和源码包的 GPG 签名

#### sha512sum

使用 `sha512sum -c` 验证二进制包和源码包的 SHA-512 校验和

#### check-license

解压二进制包，检查 server / namingserver 模块的 LICENSE 文件内容

#### check-notice

解压二进制包，校验 server / namingserver 模块的 NOTICE 文件：

- 第 1 行：`Apache Seata (Incubating)`
- 第 2 行：`Copyright 2023-{当前年份} The Apache Software Foundation`

#### check-compile-license

从不同源码来源（zip / tag / branch）获取源码，在 JDK 8 / 17 / 21 / 25 上编译并导出依赖树。 Go 程序仅在 JDK 25 中运行，检查
LICENSE 文件是否包含依赖树中的每个依赖

#### check-eyes-license

从不同源码来源（zip / tag / branch）获取源码，使用 Apache SkyWalking-Eyes 检查许可证头和依赖许可证合规性

#### check-sha

Checkout tag / branch，验证其指向的 commit SHA 是否与期望值一致

## [main.go](main.go)

解析 `mvn dependency:tree` 输出的依赖树，解析每个依赖的 Maven 坐标并从本地仓库读取 POM 文件提取许可证信息，然后检查每个依赖是否在
LICENSE 白名单文件中声明。

### 用法

```shell
go run main.go --file=tree.txt --license-file=LICENSE --exclude-group=org.apache.seata --pom-build=pom.xml
```

### 参数

| 参数                 | 短名  | 默认值     | 说明                                       |
|----------------------|-------|------------|--------------------------------------------|
| `--file`             | `-f`  | `tree.txt` | `mvn dependency:tree` 的输出文件           |
| `--license-file`     | `-lf` | `LICENSE`  | 许可证白名单文件路径                       |
| `--exclude-group`    | `-eg` | —          | 排除的 Maven groupId（可重复）             |
| `--exclude-artifact` | `-ea` | —          | 排除的 Maven groupId:artifactId（可重复）  |
| `--check-version`    | `-cv` | `true`     | 匹配时是否包含版本号                       |
| `--skip-test`        | `-st` | `false`    | 跳过 test scope 的依赖                     |
| `--pom-build`        | `-pb` | —          | 用于解析 pom.xml 的 build 段，自动排除依赖 |

### 流程

1. 解析 `mvn dependency:tree` 输出中的依赖行（以 `+- ` 或 `\- ` 开头）
2. 解析 Maven 坐标（groupId:artifactId:type:version:scope）
3. 从本地 Maven 仓库读取对应 POM 文件，提取许可证信息（如 POM 未声明则递归查找父 POM）
4. 去重、排除指定 groupId 和 groupId:artifactId，根据 `--pom-build` 中
   `maven-dependency-plugin` 配置自动排除 groupId，在白名单文件中逐个子串匹配
5. 输出所有未命中依赖并返回错误
