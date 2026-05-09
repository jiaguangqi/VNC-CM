import React, { useState, useEffect, useCallback, useRef } from "react";
import {
  Modal, Button, Table, Space, Empty, message,
  Popconfirm, Input, Tooltip, Spin, Badge, Progress,
} from "antd";
import {
  FolderOutlined, FileTextOutlined, ArrowDownOutlined,
  HomeOutlined, PlusOutlined, ReloadOutlined,
  DeleteOutlined, DesktopOutlined, CloudUploadOutlined,
  UploadOutlined, MinusOutlined, CloseOutlined,
} from "@ant-design/icons";
import { fileAPI } from "../api";
import { useFileTransferStore } from "../stores/fileTransferStore";

/* =========================================================
   类型
   ========================================================= */

interface RemoteEntry {
  name: string;
  path: string;
  size: number;
  mode: string;
  mod_time: string;
  is_dir: boolean;
}

/* =========================================================
   工具
   ========================================================= */

function fmtBytes(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB", "TB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + " " + sizes[i];
}

/** 监听窗口尺寸 */
function useWindowSize() {
  const [size, setSize] = useState({
    width: window.innerWidth,
    height: window.innerHeight,
  });
  useEffect(() => {
    const onResize = () =>
      setSize({ width: window.innerWidth, height: window.innerHeight });
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, []);
  return size;
}

/* =========================================================
   递归读取本地目录（DataTransferItem API）
   ========================================================= */

async function traverseDirectory(
  entry: FileSystemDirectoryEntry,
  path = ""
): Promise<{ relativePath: string; file: File }[]> {
  const results: { relativePath: string; file: File }[] = [];
  const reader = entry.createReader();

  const readEntries = (): Promise<FileSystemEntry[]> =>
    new Promise((resolve) => reader.readEntries(resolve));

  let entries: FileSystemEntry[];
  do {
    entries = await readEntries();
    for (const e of entries) {
      if (e.isFile) {
        const f = await new Promise<File>((res) =>
          (e as FileSystemFileEntry).file(res)
        );
        results.push({
          relativePath: path ? `${path}/${e.name}` : e.name,
          file: f,
        });
      } else if (e.isDirectory) {
        const sub = await traverseDirectory(
          e as FileSystemDirectoryEntry,
          path ? `${path}/${e.name}` : e.name
        );
        results.push(...sub);
      }
    }
  } while (entries.length > 0);

  return results;
}

/* =========================================================
   组件
   ========================================================= */

const FileTransferModal: React.FC = () => {
  const store = useFileTransferStore();
  const { visible, desktopId, desktopName, minimize, close, addTask, updateTask } = store;

  const [remotePath, setRemotePath] = useState<string>(".");
  const [remoteEntries, setRemoteEntries] = useState<RemoteEntry[]>([]);
  const [remoteLoading, setRemoteLoading] = useState(false);
  const [selectedRemoteKeys, setSelectedRemoteKeys] = useState<React.Key[]>([]);

  const [uploading, setUploading] = useState(false);
  const [uploadDone, setUploadDone] = useState(0);
  const [uploadTotal, setUploadTotal] = useState(0);
  const [dragOver, setDragOver] = useState(false);

  const [mkdirOpen, setMkdirOpen] = useState(false);
  const [newDirName, setNewDirName] = useState("");
  const [downloadProgress, setDownloadProgress] = useState<Record<string, number>>({});

  const fileInputRef = useRef<HTMLInputElement>(null);
  const winSize = useWindowSize();

  /* ---- 自适应尺寸 ---- */
  const modalWidth = Math.min(Math.max(winSize.width * 0.82, 720), 1400);
  const bodyHeight = Math.min(Math.max(winSize.height * 0.7, 400), 860);
  const listHeight = bodyHeight - 120; // 扣除工具栏 + 底部进度条

  /* =======================================================
     加载远程目录
     ======================================================= */
  const loadRemote = useCallback(async () => {
    if (!desktopId) return;
    setRemoteLoading(true);
    try {
      const res = await fileAPI.list(desktopId, remotePath);
      setRemoteEntries(res.data.entries || []);
    } catch (err: any) {
      message.error(err.response?.data?.error || "加载远程目录失败");
    } finally {
      setRemoteLoading(false);
    }
  }, [desktopId, remotePath]);

  useEffect(() => {
    if (visible) loadRemote();
  }, [visible, loadRemote]);

  /* =======================================================
     远程导航
     ======================================================= */
  const enterRemoteDir = (entry: RemoteEntry) => {
    if (!entry.is_dir) return;
    setRemotePath(entry.path);
    setSelectedRemoteKeys([]);
  };

  const goUpRemote = () => {
    if (remotePath === ".") return;
    const parts = remotePath.split("/").filter(Boolean);
    parts.pop();
    setRemotePath(parts.length === 0 ? "." : parts.join("/"));
    setSelectedRemoteKeys([]);
  };

  /* =======================================================
     上传核心
     ======================================================= */
  const doUpload = async (file: File, relativePath: string) => {
    const taskId = addTask({
      type: "upload",
      filename: relativePath,
      progress: 0,
      status: "running",
    });
    try {
      await fileAPI.upload(desktopId!, remotePath, file, relativePath);
      updateTask(taskId, { progress: 100, status: "done" });
    } catch (err: any) {
      updateTask(taskId, {
        status: "error",
        error: err.response?.data?.error || "上传失败",
      });
      message.error(`${relativePath} 上传失败`);
      throw err;
    }
  };

  const processFiles = async (
    items: { file: File; relativePath: string }[]
  ) => {
    if (items.length === 0) return;
    setUploading(true);
    setUploadDone(0);
    setUploadTotal(items.length);
    let ok = 0;
    for (let i = 0; i < items.length; i++) {
      const { file, relativePath } = items[i];
      try {
        await doUpload(file, relativePath);
        ok++;
      } catch {
        /* 已在 doUpload 中提示 */
      }
      setUploadDone(i + 1);
    }
    setUploading(false);
    message.success(`上传完成（${ok}/${items.length}）`);
    loadRemote();
    setSelectedRemoteKeys([]);
  };

  /* ---- 按钮上传 ---- */
  const handleFileInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files;
    if (!files || files.length === 0) return;
    const items: { file: File; relativePath: string }[] = [];
    for (const f of Array.from(files)) {
      const wrp = f.webkitRelativePath || "";
      if (wrp) {
        const parts = wrp.split("/");
        parts.shift();
        items.push({ file: f, relativePath: parts.join("/") });
      } else {
        items.push({ file: f, relativePath: f.name });
      }
    }
    processFiles(items);
    e.target.value = "";
  };

  /* ---- 拖拽上传 ---- */
  const onDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setDragOver(true);
  };
  const onDragLeave = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setDragOver(false);
  };

  const onDrop = async (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setDragOver(false);

    const items: { file: File; relativePath: string }[] = [];
    const dt = e.dataTransfer;
    if (!dt.items || dt.items.length === 0) return;

    for (let i = 0; i < dt.items.length; i++) {
      const item = dt.items[i];
      const entry = item.webkitGetAsEntry && item.webkitGetAsEntry();
      if (!entry) continue;

      if (entry.isFile) {
        const f = await new Promise<File>((res) =>
          (entry as FileSystemFileEntry).file(res)
        );
        items.push({ file: f, relativePath: f.name });
      } else if (entry.isDirectory) {
        const subItems = await traverseDirectory(
          entry as FileSystemDirectoryEntry,
          entry.name
        );
        items.push(...subItems);
      }
    }

    if (items.length === 0) {
      message.warning("未检测到可上传的文件");
      return;
    }
    processFiles(items);
  };

  /* =======================================================
     批量下载
     ======================================================= */
  const handleDownload = async () => {
    if (selectedRemoteKeys.length === 0) {
      message.warning("请勾选要下载的文件");
      return;
    }
    for (const key of selectedRemoteKeys) {
      const entry = remoteEntries.find((e) => e.path === key);
      if (!entry) continue;
      const taskId = addTask({
        type: "download",
        filename: entry.name,
        progress: 0,
        status: "running",
      });
      try {
        const res = await fileAPI.download(desktopId!, entry.path);
        const blob = new Blob([res.data]);
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = entry.is_dir ? `${entry.name}.zip` : entry.name;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        window.URL.revokeObjectURL(url);
        updateTask(taskId, { progress: 100, status: "done" });
        setDownloadProgress((prev) => ({ ...prev, [taskId]: 100 }));
      } catch {
        updateTask(taskId, { status: "error", error: "下载失败" });
        message.error(`下载 ${entry.name} 失败`);
      }
    }
  };

  /* =======================================================
     远程：删除 / 新建文件夹
     ======================================================= */
  const handleRemoteDelete = async (paths: string[]) => {
    let ok = 0;
    let fail = 0;
    for (const p of paths) {
      try {
        await fileAPI.del(desktopId!, p);
        ok++;
      } catch (err: any) {
        fail++;
        message.error(err.response?.data?.error || "删除失败: " + p);
      }
    }
    if (ok > 0 && fail === 0) {
      message.success(`删除成功（${ok} 项）`);
    } else if (ok > 0 && fail > 0) {
      message.warning(`部分删除成功：成功 ${ok} 项，失败 ${fail} 项`);
    } else {
      message.error("删除失败");
    }
    setSelectedRemoteKeys([]);
    loadRemote();
  };

  const handleMkdir = async () => {
    if (!newDirName.trim()) {
      message.warning("请输入文件夹名称");
      return;
    }
    const target = remotePath === "." ? newDirName : `${remotePath}/${newDirName}`;
    try {
      await fileAPI.mkdir(desktopId!, target);
      message.success("创建成功");
      setMkdirOpen(false);
      setNewDirName("");
      loadRemote();
    } catch (err: any) {
      message.error(err.response?.data?.error || "创建失败");
    }
  };

  /* =======================================================
     表格列
     ======================================================= */
  const remoteColumns = [
    {
      title: "名称",
      dataIndex: "name",
      key: "name",
      render: (_: any, r: RemoteEntry) => (
        <Space>
          {r.is_dir ? (
            <FolderOutlined style={{ color: "#faad14" }} />
          ) : (
            <FileTextOutlined style={{ color: "#52c41a" }} />
          )}
          {r.is_dir ? (
            <a onClick={() => enterRemoteDir(r)}>{r.name}</a>
          ) : (
            <span>{r.name}</span>
          )}
        </Space>
      ),
    },
    {
      title: "大小",
      dataIndex: "size",
      key: "size",
      width: 100,
      render: (s: number, r: RemoteEntry) => (r.is_dir ? "-" : fmtBytes(s)),
    },
    {
      title: "修改时间",
      dataIndex: "mod_time",
      key: "mod_time",
      width: 160,
      render: (t: string) => new Date(t).toLocaleString(),
    },
  ];

  /* =======================================================
     面包屑
     ======================================================= */
  const renderBreadcrumb = (currentPath: string) => {
    if (currentPath === ".") {
      return (
        <span style={{ fontSize: 13 }}>
          <HomeOutlined /> 家目录
        </span>
      );
    }
    const parts = currentPath.split("/").filter(Boolean);
    return (
      <span style={{ fontSize: 13 }}>
        <a
          onClick={() => {
            setRemotePath(".");
            setSelectedRemoteKeys([]);
          }}
        >
          <HomeOutlined /> 家目录
        </a>
        {parts.map((p, i) => (
          <span key={i}>
            <span style={{ margin: "0 4px", color: "#ccc" }}>/</span>
            <a
              onClick={() => {
                const np = parts.slice(0, i + 1).join("/");
                setRemotePath(np);
                setSelectedRemoteKeys([]);
              }}
            >
              {p}
            </a>
          </span>
        ))}
      </span>
    );
  };

  /* =======================================================
     底部任务进度面板
     ======================================================= */
  const tasks = store.tasks;
  const activeTasks = tasks.filter((t) => t.status === "running");
  const doneCount = tasks.filter((t) => t.status === "done").length;
  const errorCount = tasks.filter((t) => t.status === "error").length;

  /* =======================================================
     渲染
     ======================================================= */
  if (!visible) return null;

  return (
    <Modal
      title={
        <Space style={{ fontSize: 16 }}>
          <DesktopOutlined /> {desktopName} — 文件传输
        </Space>
      }
      open={visible}
      onCancel={minimize}
      width={modalWidth}
      footer={null}
      closable={false}
      maskClosable={false}
      bodyStyle={{ padding: 0, overflow: "hidden" }}
      styles={{
        body: { padding: 0, overflow: "hidden", height: bodyHeight },
      }}
    >
      {/* 自定义标题栏右上角按钮 */}
      <div
        style={{
          position: "absolute",
          top: 12,
          right: 12,
          zIndex: 20,
          display: "flex",
          gap: 4,
        }}
      >
        <Tooltip title="最小化（后台继续传输）">
          <Button
            size="small"
            icon={<MinusOutlined />}
            onClick={minimize}
          />
        </Tooltip>
        <Tooltip title="关闭">
          <Button
            size="small"
            danger
            icon={<CloseOutlined />}
            onClick={close}
          />
        </Tooltip>
      </div>

      {/* 主体内容 —— 拖拽区域 */}
      <div
        onDragOver={onDragOver}
        onDragLeave={onDragLeave}
        onDrop={onDrop}
        style={{
          border: dragOver ? "2px dashed #1890ff" : "2px dashed transparent",
          background: dragOver
            ? "rgba(24, 144, 255, 0.05)"
            : "transparent",
          borderRadius: 4,
          transition: "all 0.2s",
          padding: 12,
          height: bodyHeight,
          display: "flex",
          flexDirection: "column",
        }}
      >
        {/* 顶部工具栏 */}
        <div
          style={{
            padding: "10px 16px",
            borderBottom: "1px solid #f0f0f0",
            background: "#fafafa",
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
            flexWrap: "wrap",
            gap: 8,
            flexShrink: 0,
          }}
        >
          <Space size="small" wrap>
            {renderBreadcrumb(remotePath)}
            {remotePath !== "." && (
              <Tooltip title="上级目录">
                <Button size="small" icon={<HomeOutlined />} onClick={goUpRemote}>
                  ..
                </Button>
              </Tooltip>
            )}
          </Space>

          <Space size="small" wrap>
            {uploading && (
              <Badge
                count={`${uploadDone}/${uploadTotal}`}
                style={{ backgroundColor: "#1890ff" }}
                overflowCount={999}
              />
            )}

            <Button
              type="primary"
              size="small"
              icon={<CloudUploadOutlined />}
              loading={uploading}
              onClick={() => fileInputRef.current?.click()}
            >
              上传
            </Button>
            <input
              ref={fileInputRef}
              type="file"
              multiple
              webkitdirectory=""
              directory=""
              style={{ display: "none" }}
              onChange={handleFileInputChange}
            />

            <Button
              size="small"
              icon={<ArrowDownOutlined />}
              disabled={selectedRemoteKeys.length === 0}
              onClick={handleDownload}
            >
              下载 ({selectedRemoteKeys.length})
            </Button>

            <Button
              icon={<ReloadOutlined />}
              size="small"
              onClick={loadRemote}
            />
            <Button
              icon={<PlusOutlined />}
              size="small"
              onClick={() => setMkdirOpen(true)}
            />
            <Popconfirm
              title={`确定删除选中的 ${selectedRemoteKeys.length} 项？`}
              onConfirm={() =>
                handleRemoteDelete(selectedRemoteKeys as string[])
              }
              disabled={selectedRemoteKeys.length === 0}
            >
              <Button
                icon={<DeleteOutlined />}
                size="small"
                danger
                disabled={selectedRemoteKeys.length === 0}
              >
                删除
              </Button>
            </Popconfirm>
          </Space>
        </div>

        {/* 拖拽遮罩提示 */}
        {dragOver && (
          <div
            style={{
              position: "absolute",
              top: 60,
              left: 16,
              right: 16,
              bottom: 16,
              background: "rgba(24, 144, 255, 0.08)",
              border: "2px dashed #1890ff",
              borderRadius: 8,
              display: "flex",
              flexDirection: "column",
              alignItems: "center",
              justifyContent: "center",
              pointerEvents: "none",
              zIndex: 10,
            }}
          >
            <CloudUploadOutlined
              style={{ fontSize: 48, color: "#1890ff", marginBottom: 12 }}
            />
            <div
              style={{ fontSize: 16, color: "#1890ff", fontWeight: "bold" }}
            >
              松开鼠标上传文件
            </div>
            <div style={{ fontSize: 12, color: "#666", marginTop: 4 }}>
              支持文件和文件夹拖拽，保留完整目录结构
            </div>
          </div>
        )}

        {/* 文件列表 */}
        <div
          style={{
            flex: 1,
            overflow: "hidden",
            position: "relative",
            minHeight: 0,
          }}
        >
          <Spin
            spinning={remoteLoading}
            style={{ height: "100%", width: "100%" }}
          >
            <div style={{ height: "100%", overflow: "auto", padding: 8 }}>
              {remoteEntries.length === 0 ? (
                <Empty
                  description={
                    <div style={{ textAlign: "center" }}>
                      <div style={{ fontSize: 24, marginBottom: 8 }}>📂</div>
                      <div>当前目录为空</div>
                      <div
                        style={{
                          fontSize: 12,
                          color: "#999",
                          marginTop: 4,
                          lineHeight: 1.6,
                        }}
                      >
                        拖拽本地文件/文件夹到此处上传
                        <br />
                        或点击「上传」按钮选择文件/文件夹
                      </div>
                    </div>
                  }
                />
              ) : (
                <Table
                  dataSource={remoteEntries}
                  columns={remoteColumns}
                  rowKey="path"
                  pagination={false}
                  size="small"
                  rowSelection={{
                    selectedRowKeys: selectedRemoteKeys,
                    onChange: setSelectedRemoteKeys,
                  }}
                  onRow={(r) => ({
                    onDoubleClick: () => {
                      if (r.is_dir) enterRemoteDir(r);
                    },
                  })}
                  scroll={{ y: listHeight - 60 }}
                />
              )}
            </div>
          </Spin>
        </div>

        {/* 底部任务进度面板 */}
        {(activeTasks.length > 0 || doneCount > 0 || errorCount > 0) && (
          <div
            style={{
              flexShrink: 0,
              padding: "8px 16px",
              borderTop: "1px solid #f0f0f0",
              background: "#fafafa",
              fontSize: 12,
              maxHeight: 140,
              overflowY: "auto",
            }}
          >
            <div
              style={{
                display: "flex",
                justifyContent: "space-between",
                alignItems: "center",
                marginBottom: 6,
              }}
            >
              <Space>
                {uploading ? (
                  <>
                    <UploadOutlined spin />
                    <span>
                      正在上传 {uploadDone}/{uploadTotal}
                    </span>
                  </>
                ) : (
                  <span style={{ color: "#888" }}>传输任务</span>
                )}
                {doneCount > 0 && (
                  <Badge
                    count={doneCount}
                    style={{ backgroundColor: "#52c41a" }}
                  />
                )}
                {errorCount > 0 && (
                  <Badge
                    count={errorCount}
                    style={{ backgroundColor: "#f5222d" }}
                  />
                )}
              </Space>
              {doneCount > 0 && (
                <Button size="small" onClick={() => store.clearDone()}>
                  清除已完成
                </Button>
              )}
            </div>

            <div style={{ display: "flex", flexWrap: "wrap", gap: 6 }}>
              {tasks.slice(-8).map((t) => (
                <div
                  key={t.id}
                  style={{
                    flex: "1 1 280px",
                    minWidth: 200,
                  }}
                >
                  <div
                    style={{
                      display: "flex",
                      justifyContent: "space-between",
                      marginBottom: 2,
                    }}
                  >
                    <span
                      style={{
                        fontSize: 11,
                        color:
                          t.status === "error"
                            ? "#f5222d"
                            : t.status === "done"
                            ? "#52c41a"
                            : "#333",
                        overflow: "hidden",
                        textOverflow: "ellipsis",
                        whiteSpace: "nowrap",
                        maxWidth: "70%",
                      }}
                      title={t.filename}
                    >
                      {t.type === "upload" ? "↑" : "↓"} {t.filename}
                    </span>
                    <span style={{ fontSize: 11, color: "#999" }}>
                      {t.status === "done"
                        ? "完成"
                        : t.status === "error"
                        ? "失败"
                        : `${t.progress}%`}
                    </span>
                  </div>
                  {t.status === "running" && (
                    <Progress
                      percent={t.progress}
                      size="small"
                      showInfo={false}
                      strokeColor="#1890ff"
                    />
                  )}
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* 新建文件夹弹窗 */}
      <Modal
        title="新建文件夹"
        open={mkdirOpen}
        onOk={handleMkdir}
        onCancel={() => {
          setMkdirOpen(false);
          setNewDirName("");
        }}
      >
        <Input
          placeholder="文件夹名称"
          value={newDirName}
          onChange={(e) => setNewDirName(e.target.value)}
          onPressEnter={handleMkdir}
        />
      </Modal>
    </Modal>
  );
};

export default FileTransferModal;
