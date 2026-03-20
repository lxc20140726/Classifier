# 配置系统设计 v3.0

> 版本：v3.0 | 日期：2026-03-20

## 目标

- 强类型配置
- 默认值
- 校验
- 版本化迁移
- 动态重载

## 存储模型

```sql
CREATE TABLE app_config (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  version INTEGER NOT NULL,
  value TEXT NOT NULL,
  checksum TEXT NOT NULL,
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
```

## 加载顺序

1. defaults
2. app_config.value
3. env overrides
4. runtime patch

## 迁移接口

```go
type Migrator interface {
  FromVersion() int
  ToVersion() int
  Migrate(raw []byte) ([]byte, error)
}
```

## 热更新

- 配置保存成功后触发 reload hooks
- 典型 hook：scanner interval、max concurrency、snapshot retention
