# 项目开发规则（AI 必读）

## 前端改动必须重新构建
- `npx vue-tsc --noEmit` 只做类型检查，不产出文件。
- Wails 运行时加载的是 `frontend/dist/`，改完 `frontend/src/` 后必须执行：
  ```bash
  cd frontend && npm run build
  ```
- 若运行的是已打包的 `wails build` 可执行文件，还需重新 `wails build` 并重启程序。

## 校验命令
```bash
cd frontend && npx vue-tsc --noEmit   # 类型检查
cd frontend && npm run build          # 必须，产出 dist
go build ./...                        # 后端编译
```

## 其他约定
- Wails 绑定：`Call.ByName('main.App.<方法名>', ...args)`，统一封装在 `frontend/src/api.ts`。
- 前端 i18n key 前缀分组：`common.*` / `editor.*` / `menu.*` 等，中英文都要补。