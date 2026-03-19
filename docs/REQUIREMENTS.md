# Project Requirements / 项目需求

## Chinese

- 自动分类：支持写真、视频、混合、漫画等内容类型，支持用户自定义规则和手动指定内容类型。
- 支持可视化工作流：
    - 用户可以通过可视化界面配置分类规则，查看分类结果并进行调整
    - 支持用户修改处理文件夹工作流程，为不同分类自定义不同的工作流程
- 批量重命名：支持用户自定义规则和预设脚本。保证不懂正则表达式的用户也能快速上手
- 批量压缩：仅针对图片目录生成 ZIP 文件，强调快速压缩。
- 视频缩略图生成：输出符合 Emby 规范的缩略图资源。
- 文件移动：支持将处理后的文件移动到指定目录。
- 部署方式：以 Docker 形式部署在 NAS 环境中。
- 并发处理：可同时处理多个文件夹任务。
- 安全优先：保证文件操作的安全性，避免数据丢失或损坏。
    - 可回退操作：用户可以在操作后回退到前一个状态，避免不可逆操作。
    - 日志记录：记录所有文件操作，包括分类、重命名、压缩和移动，方便用户审计和问题排查。

## English

- Automatic classification for photo sets, videos, mixed folders, and comics.
- Batch renaming with user-defined rules and preset scripts.
- Batch compression using ZIP for image-only directories with fast turnaround.
- Video thumbnail generation that follows Emby conventions.
- File moving to user-selected target directories.
- Deployment on NAS through Docker.
- Concurrent processing for multiple folders at the same time.

## Product Direction

This project is a web-based file organization tool designed for NAS deployments. It focuses on reliable media classification, fast batch operations, and practical automation for home server workflows.
