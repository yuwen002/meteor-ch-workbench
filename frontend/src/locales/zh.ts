export default {
  app: { name: "Meteor CH 工作台", tagline: "跨平台 ClickHouse GUI 工具" },
  menu: {
    file: "文件", newConnection: "新建连接", newFolder: "新建分组",
    openConfigFolder: "打开配置目录", quit: "退出",
    edit: "编辑", undo: "撤销", redo: "重做", selectAll: "全选",
    view: "视图", toggleSidebar: "切换侧边栏", resetSidebar: "重置侧边栏宽度",
    language: "语言",
    help: "帮助", shortcuts: "快捷键", about: "关于"
  },
  shortcuts: {
    title: "快捷键", action: "操作", shortcut: "快捷键",
    newConnection: "新建连接", newFolder: "新建分组",
    runQuery: "执行查询", toggleSidebar: "切换侧边栏",
    focusEditor: "聚焦编辑器", selectAll: "全选", undo: "撤销", redo: "重做"
  },
  about: {
    title: "关于 Meteor CH Workbench", version: "版本",
    desc: "基于 Wails v3、Vue 3 和 Go 构建的轻量级跨平台 ClickHouse 图形化客户端。",
    close: "关闭"
  },
  sidebar: {
    new: "新建", newConnection: "新建连接", newFolder: "新建分组",
    noConnections: "暂无连接。", clickNewToAdd: "点击 新建 按钮添加。",
    ungrouped: "未分组", expand: "展开", collapse: "收起",
    rename: "重命名", delete: "删除", emptyFolder: "暂无连接"
  },
  connection: {
    titleNew: "新建连接", titleEdit: "编辑连接", name: "名称", folder: "所属分组",
    noFolder: "— 不分组 —", host: "主机", port: "端口", user: "用户",
    password: "密码", database: "数据库", secure: "加密连接 (TLS)", compress: "数据压缩", insecure: "跳过证书校验（不安全）",
    insecureHint: "仅在使用自签证书的内网集群时勾选。开启后 TLS 不再验证服务端身份，存在被中间人窃取账号密码的风险。",
    testConnection: "测试连接", testing: "测试中...", create: "创建", save: "保存",
    cancel: "取消", deleteBtn: "删除", confirmDelete: "确定删除此连接？",
    testSuccess: "连接成功！", testFailed: "连接失败"
  },
  folder: {
    titleNew: "新建分组", titleEdit: "重命名分组", name: "分组名称",
    create: "创建", rename: "重命名", cancel: "取消",
    confirmDelete: "确定删除此分组？分组内的连接将变为未分组。"
  },
  editor: {
    selectConnection: "请先从左侧选择一个连接开始", run: "执行 (Ctrl+Enter)",
    running: "执行中...", refresh: "刷新", databases: "数据库",
    noDatabases: "暂无数据库", loading: "加载中...", results: "查询结果",
    rows: "行", noResults: "查询完成，无结果",
    queryHint: "在上方输入 SQL，点击执行 (Ctrl+Enter) 运行",
    noConnection: "请先选择一个连接", queryFailed: "查询失败",
    clusters: "集群", noClusters: "暂无集群信息",
    copyIdentifier: "复制副本标识", copied: "已复制到剪贴板",
    copyCluster: "复制集群名", insertCluster: "插入集群名",
    errors: "连接失败次数",
    shard: "分片", replica: "副本", local: "本地"
  },
  common: { close: "关闭", unknown: "未知" }
}