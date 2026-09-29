# 迁移状态

[English](migration-todo.md) | 简体中文

计划内 Playa 迁移已经完成。

本文件为历史链接保留稳定目标，不再是活动中的 checkbox backlog。当前维护以
以下文件为真源：

- [`compat/upstream.toml`](../compat/upstream.toml) 标识固定的 Playa oracle；
  文档不复制其版本号。
- [`compat/manifest.toml`](../compat/manifest.toml) 定义参与比较的公开 API
  section；当前每个 section 都是 required。
- [`migration.zh-CN.md`](migration.zh-CN.md) 定义范围、接口适配、惰性、所有权、
  生命周期和有意差异。
- [`compatibility.zh-CN.md`](compatibility.zh-CN.md) 定义 corpus 比较和发布验收
  工作流。
- [`engineering.zh-CN.md`](engineering.zh-CN.md) 定义后续变更门禁。

已完成范围包括 PDF 解析与恢复、加密、页面和对象遍历、内容解释、布局、字体与
CMap、图像、注释、目的地和动作、大纲、表单、tagged PDF 结构、检查 CLI、
有界并发以及最终公开领域包归属。

新的生产器特有失败属于缺陷或 corpus 加固工作，不代表迁移尚未完成。新接受的
Playa API 必须先映射到兼容清单和迁移契约，再实现所需行为、所有权与 oracle
测试。
