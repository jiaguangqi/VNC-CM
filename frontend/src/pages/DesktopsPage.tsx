import React, { useState, useEffect } from "react";
import {
  Card,
  Button,
  Table,
  Tag,
  Space,
  Modal,
  Form,
  Select,
  message,
  Typography,
  Empty,
  Tabs,
  Input,
  Row,
  Col,
  Checkbox,
  Tooltip,
  Alert,
} from "antd";
import {
  PlusOutlined,
  PlayCircleOutlined,
  StopOutlined,
  DeleteOutlined,
  InboxOutlined,
  LinkOutlined,
  GlobalOutlined,
  UnorderedListOutlined,
  AppstoreOutlined,
  TeamOutlined,
  LinuxOutlined,
  WindowsOutlined,
  CheckSquareOutlined,
  CloseSquareOutlined,
  LogoutOutlined,
  CopyOutlined,
  FullscreenOutlined,
  ReloadOutlined,
} from "@ant-design/icons";
import { desktopAPI, collaborationAPI, hostAPI } from "../api";
import FileTransferModal from "../components/FileTransferModal";
import FloatingTransferStatus from "../components/FloatingTransferStatus";
import { useFileTransferStore } from "../stores/fileTransferStore";
import { useAuthStore } from "../stores/authStore";

const { Text } = Typography;
const { TabPane } = Tabs;

interface DesktopSession {
  id: string;
  display_name?: string;
  purpose?: string;
  protocol: string;
  resolution: string;
  performance_profile?: string;
  current_bandwidth_bps?: number;
  peak_bandwidth_bps?: number;
  total_network_bytes?: number;
  status: string;
  username?: string;
  host_id: string;
  host_ip: string;
  host_name: string;
  port: number;
  host_os?: string;
  vnc_password?: string;
  connection_info?: {
    url: string;
    web_url: string;
    password: string;
    port: number;
    display: number;
    error?: string;
    last_health_check_at?: string;
  };
  created_at: string;
  updated_at?: string;
}

interface HostOption {
  id: string;
  hostname: string;
  ip_address: string;
  status: string;
  current_sessions: number;
  max_sessions: number;
  region?: string;
  az?: string;
  cpu_cores?: number;
  total_ram_mb?: number;
  ready: boolean;
  current_user_exists: boolean;
  agent_managed?: boolean;
  missing?: string[];
}

type ViewMode = "list" | "grid";

const isProcessingStatus = (status: string) => ["pending", "starting", "stopping"].includes(status);
const canConnectDesktop = (status: string) => status === "running";
const canInviteDesktop = (status: string) => status === "running";
const canStopDesktop = (status: string) => !["terminated", "stopping"].includes(status);
const canBatchTerminateDesktop = (status: string) => ["pending", "starting", "running", "stopping", "error"].includes(status);

const formatCardBandwidth = (bps?: number) => {
  if (!bps || bps <= 0) return "0 bps";
  if (bps >= 1_000_000) return `${(bps / 1_000_000).toFixed(1)} Mbps`;
  if (bps >= 1_000) return `${(bps / 1_000).toFixed(1)} Kbps`;
  return `${Math.round(bps)} bps`;
};

const desktopStatusColor = (s: string) => {
  switch (s) {
    case "running": return "green";
    case "starting": return "blue";
    case "pending": return "gold";
    case "stopping": return "orange";
    case "terminated": return "default";
    case "error": return "red";
    default: return "default";
  }
};

const desktopStatusText = (s: string) => {
  switch (s) {
    case "running": return "运行中";
    case "starting": return "启动中";
    case "pending": return "等待中";
    case "stopping": return "关闭中";
    case "terminated": return "已关闭";
    case "error": return "异常";
    default: return s;
  }
};

const desktopProfileText = (profile?: string) => {
  switch (profile) {
    case "quality": return "画质优先";
    case "low_bandwidth": return "低带宽";
    case "balanced":
    default:
      return "均衡";
  }
};

const cardShellStyle: React.CSSProperties = {
  display: "flex",
  flexWrap: "wrap",
  alignItems: "flex-start",
  gap: 14,
};

const cardMetaStyle: React.CSSProperties = {
  flex: "1 1 220px",
  minWidth: 0,
  display: "grid",
  gridTemplateColumns: "repeat(auto-fit, minmax(108px, 1fr))",
  gap: "8px 14px",
  fontSize: 11.5,
  color: "#4e5969",
  lineHeight: 1.7,
};

const cardActionStyle: React.CSSProperties = {
  flex: "0 1 152px",
  minWidth: 126,
  marginLeft: "auto",
};

const thumbnailFrameStyle: React.CSSProperties = {
  width: "100%",
  aspectRatio: "16 / 9",
  borderRadius: 8,
  overflow: "hidden",
  border: "4px solid #ffffff",
  boxSizing: "border-box",
  background: "#0b1220",
  marginBottom: 12,
  position: "relative",
  zIndex: 1,
  cursor: "zoom-in",
  transformOrigin: "center center",
  transition: "transform 180ms ease, box-shadow 180ms ease, outline-color 180ms ease",
  outline: "1px solid #dbe7f3",
  boxShadow: "0 8px 22px rgba(15, 23, 42, 0.10)",
  willChange: "transform",
};

const thumbnailImageStyle: React.CSSProperties = {
  width: "100%",
  height: "100%",
  objectFit: "cover",
  display: "block",
};

const thumbnailPlaceholderStyle: React.CSSProperties = {
  width: "100%",
  height: "100%",
  display: "flex",
  flexDirection: "column",
  alignItems: "center",
  justifyContent: "center",
  gap: 6,
};

const thumbnailCache = new Map<string, { src?: string; failed?: boolean; promise?: Promise<string> }>();

const DesktopThumbnail = ({ desktop }: { desktop: DesktopSession }) => {
  const [src, setSrc] = useState("");
  const [failed, setFailed] = useState(false);
  const [hovered, setHovered] = useState(false);

  useEffect(() => {
    let cancelled = false;
    setSrc("");
    setFailed(false);
    if (desktop.status !== "running") {
      return undefined;
    }

    const cached = thumbnailCache.get(desktop.id);
    if (cached?.src) {
      setSrc(cached.src);
      return undefined;
    }
    if (cached?.failed) {
      setFailed(true);
      return undefined;
    }

    const promise = cached?.promise || desktopAPI.thumbnail(desktop.id).then((res) => {
      const objectUrl = URL.createObjectURL(res.data);
      thumbnailCache.set(desktop.id, { src: objectUrl });
      return objectUrl;
    }).catch((error) => {
      thumbnailCache.set(desktop.id, { failed: true });
      throw error;
    });
    if (!cached?.promise) {
      thumbnailCache.set(desktop.id, { promise });
    }

    promise.then((objectUrl) => {
        if (cancelled) return;
        setSrc(objectUrl);
      })
      .catch(() => {
        if (!cancelled) setFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, [desktop.id, desktop.status]);

  const thumb = src ? (
    <img src={src} alt="桌面缩略图" style={thumbnailImageStyle} />
  ) : (
    <div style={thumbnailPlaceholderStyle}>
      <LinuxOutlined style={{ fontSize: 24, color: failed ? "#ff4d4f" : "#8c8c8c" }} />
      <Text type={failed ? "danger" : "secondary"} style={{ fontSize: 11 }}>
        {failed ? "缩略图不可用" : desktop.status === "running" ? "正在获取缩略图" : "桌面未运行"}
      </Text>
    </div>
  );

  const frameStyle: React.CSSProperties = src && hovered ? {
    ...thumbnailFrameStyle,
    zIndex: 30,
    transform: "translate3d(0, -18px, 0) scale(1.42)",
    outlineColor: "#ffffff",
    boxShadow: "0 24px 60px rgba(15, 23, 42, 0.30), 0 8px 20px rgba(22, 119, 255, 0.16)",
  } : thumbnailFrameStyle;

  return (
    <div
      style={frameStyle}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
    >
      {thumb}
    </div>
  );
};

interface DesktopCardProps {
  desktop: DesktopSession;
  selected: boolean;
  runtime: string;
  sentInvites: any[];
  onSelectedChange: (checked: boolean) => void;
  onConnect: () => void;
  onTerminate: () => void;
  onInvite: () => void;
  onOpenFileTransfer: () => void;
  onStopInvite: (inviteID: string) => void;
  onDelete: () => void;
}

const DesktopCard = ({
  desktop,
  selected,
  runtime,
  sentInvites,
  onSelectedChange,
  onConnect,
  onTerminate,
  onInvite,
  onOpenFileTransfer,
  onStopInvite,
  onDelete,
}: DesktopCardProps) => {
  const isRunning = desktop.status === "running";
  const isTerminated = desktop.status === "terminated";
  const canStop = canStopDesktop(desktop.status);
  const isWindows = desktop.host_os?.toLowerCase().includes("win");
  const title = desktop.display_name || desktop.host_name;
  const activeInvites = sentInvites.filter(inv => inv.session_id === desktop.id && inv.status === "active");

  return (
    <Card
      hoverable
      className="rdp-soft-card"
      style={{
        position: "relative",
        overflow: "visible",
      }}
      bodyStyle={{ padding: "14px 14px 12px" }}
    >
      <DesktopThumbnail desktop={desktop} />
      <div style={cardShellStyle}>
        <div style={{ flex: "0 0 auto", alignSelf: "center", paddingRight: 4 }}>
          <Checkbox checked={selected} onChange={(e) => onSelectedChange(e.target.checked)} />
        </div>

        <div style={{ flex: "0 1 120px", minWidth: 96, textAlign: "center" }}>
          <div style={{ marginBottom: 4 }}>
            {isWindows ? (
              <WindowsOutlined style={{ fontSize: 28, color: "#1890ff" }} />
            ) : (
              <LinuxOutlined style={{ fontSize: 28, color: "#52c41a" }} />
            )}
          </div>
          <Text strong style={{ fontSize: 14, display: "block", overflowWrap: "anywhere" }}>
            {title}
          </Text>
          {desktop.display_name && (
            <Text type="secondary" style={{ fontSize: 11, display: "block", overflowWrap: "anywhere" }}>
              {desktop.host_name}
            </Text>
          )}
          <Tag color={desktopStatusColor(desktop.status)} style={{ marginTop: 6, fontSize: 10, height: "auto", lineHeight: 1.4 }}>
            {desktopStatusText(desktop.status)}
          </Tag>
          {desktop.status === "error" && desktop.connection_info?.error && (
            <div style={{ marginTop: 6 }}>
              <Text type="danger" style={{ fontSize: 11, overflowWrap: "anywhere" }}>
                {desktop.connection_info.error}
              </Text>
            </div>
          )}
          {desktop.username && (
            <div style={{ marginTop: 4 }}>
              <Text type="secondary" style={{ fontSize: 11, overflowWrap: "anywhere" }}>{desktop.username}</Text>
            </div>
          )}
        </div>

        <div style={cardMetaStyle}>
          {desktop.purpose && <div style={{ gridColumn: "1 / -1" }}><Text type="secondary">用途:</Text> <span style={{ overflowWrap: "anywhere" }}>{desktop.purpose}</span></div>}
          <div><Text type="secondary">运行:</Text> {runtime}</div>
          <div><Text type="secondary">协议:</Text> {desktop.protocol.toUpperCase()}</div>
          <div><Text type="secondary">分辨率:</Text> {desktop.resolution}</div>
          <div><Text type="secondary">档位:</Text> {desktopProfileText(desktop.performance_profile)}</div>
          <div><Text type="secondary">当前带宽:</Text> {formatCardBandwidth(desktop.current_bandwidth_bps)}</div>
          <div><Text type="secondary">峰值带宽:</Text> {formatCardBandwidth(desktop.peak_bandwidth_bps)}</div>
          <div><Text type="secondary">端口:</Text> {desktop.port}</div>
          <div><Text type="secondary">IP:</Text> <span style={{ overflowWrap: "anywhere" }}>{desktop.host_ip}</span></div>
        </div>

        <div style={cardActionStyle}>
          <Space direction="vertical" style={{ width: "100%" }} size="small">
            <div style={{ display: "flex", gap: 6 }}>
              <Button
                type="primary"
                size="small"
                icon={<PlayCircleOutlined />}
                onClick={onConnect}
                disabled={!isRunning}
                title="连接"
              />
              <Button
                danger
                size="small"
                icon={<StopOutlined />}
                onClick={onTerminate}
                disabled={!canStop}
                title="关闭"
              />
              {canInviteDesktop(desktop.status) && (
                <Button
                  size="small"
                  icon={<TeamOutlined />}
                  onClick={onInvite}
                  title="邀请协助"
                />
              )}
              {isRunning && (
                <Button
                  size="small"
                  icon={<InboxOutlined />}
                  onClick={onOpenFileTransfer}
                  title="文件传输"
                />
              )}
            </div>
            {activeInvites.length > 0 && (
              <div style={{ marginTop: 4, padding: "6px 8px", background: "#f0f5ff", borderRadius: 6, border: "1px solid #d6e4ff", width: "100%" }}>
                <div style={{ fontSize: 9, color: "#1677ff", marginBottom: 4, fontWeight: 500 }}>活跃协助</div>
                {activeInvites.map(inv => (
                  <div key={inv.id} style={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 8, marginTop: 3 }}>
                    <span style={{ fontSize: 10 }}>{inv.invitee?.username || "未知用户"}</span>
                    <Button
                      size="small"
                      danger
                      style={{ fontSize: 11, padding: "0 6px", height: 22 }}
                      onClick={() => onStopInvite(inv.id)}
                    >
                      终止
                    </Button>
                  </div>
                ))}
              </div>
            )}
            {isTerminated && (
              <Button
                danger
                ghost
                size="small"
                icon={<DeleteOutlined />}
                block
                onClick={onDelete}
              >
                删除
              </Button>
            )}
          </Space>
        </div>
      </div>
    </Card>
  );
};

const DesktopsPage: React.FC = () => {
  const [desktops, setDesktops] = useState<DesktopSession[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [connectModal, setConnectModal] = useState<DesktopSession | null>(null);
  const [applying, setApplying] = useState(false);
  const [viewMode, setViewMode] = useState<ViewMode>("grid");
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([]);
  const [invitedCollabs, setInvitedCollabs] = useState<any[]>([]);
  const [mySentInvites, setMySentInvites] = useState<any[]>([]);
  const [availableHosts, setAvailableHosts] = useState<HostOption[]>([]);
  const [inviteModalOpen, setInviteModalOpen] = useState(false);
  const [inviteForm] = Form.useForm();
  const [inviteLoading, setInviteLoading] = useState(false);
  const [inviteDesktop, setInviteDesktop] = useState<DesktopSession | null>(null);
  const [now, setNow] = useState(Date.now());
  const [forceClosingAll, setForceClosingAll] = useState(false);
  
  const [form] = Form.useForm();
  const { user } = useAuthStore();
  const isAdmin = user?.role === "admin";
  const { open: openFileTransfer } = useFileTransferStore();

  const copyText = async (value: string, label: string = "内容") => {
    if (!value) return;
    try {
      await navigator.clipboard.writeText(value);
      message.success(`${label}已复制`);
    } catch {
      message.error("复制失败");
    }
  };

  const formatMemory = (mb?: number) => {
    if (!mb) return "-";
    return `${(mb / 1024).toFixed(1)}G`;
  };

  const formatRuntime = (createdAt: string, status: string) => {
    if (!createdAt || !["running", "starting", "stopping"].includes(status)) return "-";
    const started = new Date(createdAt).getTime();
    if (!Number.isFinite(started)) return "-";
    const totalSeconds = Math.max(0, Math.floor((now - started) / 1000));
    const days = Math.floor(totalSeconds / 86400);
    const hours = Math.floor((totalSeconds % 86400) / 3600);
    const minutes = Math.floor((totalSeconds % 3600) / 60);
    if (days > 0) return `${days}天${hours}小时`;
    if (hours > 0) return `${hours}小时${minutes}分`;
    return `${Math.max(1, minutes)}分`;
  };

  const highRecentBandwidth = desktops.some((desktop) => (desktop.current_bandwidth_bps || 0) > 5_000_000);

  const hostDisabledReason = (host: HostOption) => {
    if (!host.current_user_exists) return "当前用户不存在";
    if (!host.ready) return host.missing?.length ? `缺失 ${host.missing.join(", ")}` : "节点未就绪";
    return "";
  };

  const runningCount = desktops.filter((desktop) => desktop.status === "running").length;
  const pendingCount = desktops.filter((desktop) => isProcessingStatus(desktop.status)).length;
  const terminatedCount = desktops.filter((desktop) => desktop.status === "terminated").length;
  const activeCount = desktops.filter((desktop) => canBatchTerminateDesktop(desktop.status)).length;


    const fetchInvited = async () => {
    try {
      const res = await collaborationAPI.listInvited();
      setInvitedCollabs(res.data || []);
    } catch (e) {
      // silent
    }
  };

  const fetchMySentInvites = async () => {
    try {
      const res = await collaborationAPI.listMyInvites();
      setMySentInvites(res.data || []);
    } catch (e) {
      // silent
    }
  };

const fetchDesktops = async () => {
    setLoading(true);
    try {
      const res = await desktopAPI.list();
      setDesktops(res.data);
    } catch (e: any) {
      message.error("获取桌面列表失败");
    } finally {
      setLoading(false);
    }
  };

  const fetchAvailableHosts = async () => {
    try {
      const res = await hostAPI.listDesktopHosts();
      setAvailableHosts(res.data?.hosts || []);
    } catch {
      setAvailableHosts([]);
    }
  };

  useEffect(() => {
    fetchDesktops();
    fetchAvailableHosts();
    fetchInvited();
    fetchMySentInvites();
    const interval = setInterval(fetchDesktops, 30000);
    const inviteInterval = setInterval(fetchMySentInvites, 10000);
    const clockInterval = setInterval(() => setNow(Date.now()), 60000);
    return () => {
      clearInterval(interval);
      clearInterval(inviteInterval);
      clearInterval(clockInterval);
    };
  }, []);

  // 批量操作
  const handleBatchTerminate = () => {
    const ids = selectedRowKeys as string[];
    if (ids.length === 0) return;
    const runningIds = ids.filter((id) => {
      const d = desktops.find((x) => x.id === id);
      return d && canBatchTerminateDesktop(d.status);
    });
    if (runningIds.length === 0) {
      message.warning("没有可关闭的运行中桌面");
      return;
    }
    Modal.confirm({
      title: `确认批量关闭 ${runningIds.length} 个桌面？`,
      content: "关闭后将释放宿主机资源",
      okText: "确认关闭",
      okType: "danger",
      cancelText: "取消",
      onOk: async () => {
        let success = 0;
        let fail = 0;
        for (const id of runningIds) {
          try {
            await desktopAPI.terminate(id);
            success++;
          } catch {
            fail++;
          }
        }
        message.success(`已关闭 ${success} 个，失败 ${fail} 个`);
        setSelectedRowKeys([]);
        fetchDesktops();
      },
    });
  };

  const handleBatchDelete = () => {
    const ids = selectedRowKeys as string[];
    if (ids.length === 0) return;
    const terminatedIds = ids.filter((id) => {
      const d = desktops.find((x) => x.id === id);
      return d && d.status === "terminated";
    });
    if (terminatedIds.length === 0) {
      message.warning("没有可删除的已关闭桌面");
      return;
    }
    Modal.confirm({
      title: `确认批量删除 ${terminatedIds.length} 条记录？`,
      content: "删除后无法恢复",
      okText: "确认删除",
      okType: "danger",
      cancelText: "取消",
      onOk: async () => {
        let success = 0;
        let fail = 0;
        for (const id of terminatedIds) {
          try {
            await desktopAPI.deleteRecord(id);
            success++;
          } catch {
            fail++;
          }
        }
        message.success(`已删除 ${success} 个，失败 ${fail} 个`);
        setSelectedRowKeys([]);
        fetchDesktops();
      },
    });
  };

  const handleForceTerminateAll = () => {
    if (!isAdmin) return;
    if (activeCount === 0) {
      message.info("当前没有需要关闭的活动桌面");
      return;
    }
    Modal.confirm({
      title: `强制关闭全部 ${activeCount} 个活动桌面？`,
      content: "该操作会关闭所有用户的运行中、启动中、异常和关闭中的桌面，用于快速释放宿主机资源。",
      okText: "强制关闭全部",
      okType: "danger",
      cancelText: "取消",
      onOk: async () => {
        setForceClosingAll(true);
        try {
          const res = await desktopAPI.forceTerminateAll();
          const successCount = res.data?.successCount ?? 0;
          const failedCount = res.data?.failedCount ?? 0;
          if (failedCount > 0) {
            message.warning(`已关闭 ${successCount} 个，失败 ${failedCount} 个`);
          } else {
            message.success(`已强制关闭 ${successCount} 个桌面`);
          }
          setSelectedRowKeys([]);
          fetchDesktops();
        } catch (e: any) {
          message.error(e.response?.data?.error || "强制关闭全部桌面失败");
        } finally {
          setForceClosingAll(false);
        }
      },
    });
  };

  const handleApply = async (values: any) => {
    setApplying(true);
    try {
      const res = await desktopAPI.create({
        protocol: values.protocol,
        resolution: values.resolution,
        color_depth: values.color_depth || 24,
        desktop_env: values.desktop_env,
        vnc_backend: values.protocol === "vnc" ? (values.vnc_backend || "turbovnc") : undefined,
        performance_profile: values.performance_profile || "balanced",
        host_id: values.host_id === "auto" ? undefined : values.host_id,
        display_name: values.display_name,
        purpose: values.purpose,
      });
      message.success(
        <span>
          桌面申请成功！端口: <b>{res.data.port}</b>，密码: <b>{res.data.vnc_password}</b>
        </span>,
        5
      );
      setModalOpen(false);
      form.resetFields();
      fetchDesktops();
    } catch (e: any) {
      message.error(e.response?.data?.error || "申请桌面失败");
    } finally {
      setApplying(false);
    }
  };

  const handleTerminate = async (id: string) => {
    Modal.confirm({
      title: "确认关闭桌面？",
      content: "关闭后将释放宿主机资源",
      okText: "确认关闭",
      okType: "danger",
      cancelText: "取消",
      onOk: async () => {
        try {
          await desktopAPI.terminate(id);
          message.success("桌面已关闭");
          fetchDesktops();
        } catch (e: any) {
          message.error(e.response?.data?.error || "关闭失败");
        }
      },
    });
  };

  const handleDelete = async (id: string) => {
    Modal.confirm({
      title: "确认删除记录？",
      content: "删除后无法恢复",
      okText: "确认删除",
      okType: "danger",
      cancelText: "取消",
      onOk: async () => {
        try {
          await desktopAPI.deleteRecord(id);
          message.success("记录已删除");
          fetchDesktops();
        } catch (e: any) {
          message.error(e.response?.data?.error || "删除失败");
        }
      },
    });
  };

  const statusColor = (s: string) => {
    switch (s) {
      case "running": return "green";
      case "pending":
      case "starting": return "gold";
      case "stopping": return "orange";
      case "terminated": return "default";
      case "error": return "red";
      default: return "red";
    }
  };

  const statusText = (s: string) => {
    switch (s) {
      case "running": return "运行中";
      case "pending": return "待创建";
      case "starting": return "启动中";
      case "stopping": return "关闭中";
      case "terminated": return "已关闭";
      case "error": return "异常";
      default: return s;
    }
  };

  // 表格列定义
  const baseColumns = [
    {
      title: "名称/用途",
      dataIndex: "display_name",
      key: "display_name",
      width: 180,
      render: (_: string, record: DesktopSession) => (
        <Space direction="vertical" size={0}>
          <Text strong>{record.display_name || record.host_name}</Text>
          {record.purpose && <Text type="secondary" style={{ fontSize: 12 }}>{record.purpose}</Text>}
        </Space>
      ),
    },
    {
      title: "协议",
      dataIndex: "protocol",
      key: "protocol",
      render: (p: string) => <Tag color="blue">{p.toUpperCase()}</Tag>,
    },
    { title: "分辨率", dataIndex: "resolution", key: "resolution" },
    {
      title: "状态",
      dataIndex: "status",
      key: "status",
      render: (s: string) => <Tag color={statusColor(s)}>{statusText(s)}</Tag>,
    },
    {
      title: "宿主机",
      dataIndex: "host_name",
      key: "host_name",
      render: (h: string, record: DesktopSession) => (
        <Space>
          {record.host_os?.toLowerCase().includes("win") ? (
            <WindowsOutlined style={{ color: "#1890ff" }} />
          ) : (
            <LinuxOutlined style={{ color: "#52c41a" }} />
          )}
          {h}
        </Space>
      ),
    },
    { title: "端口", dataIndex: "port", key: "port", width: 80 },
    {
      title: "运行时长",
      key: "runtime",
      width: 100,
      render: (_: any, record: DesktopSession) => formatRuntime(record.created_at, record.status),
    },
    {
      title: "创建时间",
      dataIndex: "created_at",
      key: "created_at",
      render: (t: string) => new Date(t).toLocaleString(),
    },
    {
      title: "操作",
      key: "action",
      width: 180,
      fixed: "right",
      render: (_: any, record: DesktopSession) => (
        <Space size="small">
          <Tooltip title="连接">
            <Button
              size="small"
              type="primary"
              icon={<PlayCircleOutlined />}
              onClick={() => setConnectModal(record)}
              disabled={!canConnectDesktop(record.status)}
            />
          </Tooltip>
          <Tooltip title="文件传输">
            <Button
              size="small"
              icon={<InboxOutlined />}
              onClick={() => openFileTransfer(record.id, record.host_name || record.id)}
              disabled={!canConnectDesktop(record.status)}
            />
          </Tooltip>
          <Tooltip title="关闭">
            <Button
              size="small"
              danger
              icon={<StopOutlined />}
              onClick={() => handleTerminate(record.id)}
              disabled={!canStopDesktop(record.status)}
            />
          </Tooltip>
          {canInviteDesktop(record.status) && (
            <Tooltip title="邀请协作者">
              <Button
                size="small"
                icon={<TeamOutlined />}
                onClick={() => {
                  setInviteDesktop(record);
                  setInviteModalOpen(true);
                }}
              />
            </Tooltip>
          )}
          {record.status === "terminated" && (
            <Tooltip title="删除记录">
              <Button
                size="small"
                danger
                ghost
                icon={<DeleteOutlined />}
                onClick={() => handleDelete(record.id)}
              />
            </Tooltip>
          )}
        </Space>
      ),
    },
  ];

  const adminColumns = isAdmin
    ? [
        { title: "ID", dataIndex: "id", key: "id", ellipsis: true, width: 100 },
        {
          title: "用户",
          dataIndex: "username",
          key: "username",
          render: (u: string) => <Tag color={u === "admin" ? "red" : "default"}>{u}</Tag>,
        },
        ...baseColumns,
        { title: "宿主机IP", dataIndex: "host_ip", key: "host_ip" },
      ]
    : baseColumns;

  const rowSelection = {
    selectedRowKeys,
    onChange: (keys: React.Key[]) => setSelectedRowKeys(keys),
    selections: [
      Table.SELECTION_ALL,
      Table.SELECTION_INVERT,
      Table.SELECTION_NONE,
    ],
  };

  // 连接弹窗内容
  const ConnectModalContent = ({ desktop }: { desktop: DesktopSession }) => {
    const url = desktop.connection_info?.url || "";
    const webUrl = desktop.connection_info?.web_url || "";
    const password = desktop.connection_info?.password || "";
    const errorText = desktop.connection_info?.error || "";
    const openWebDesktop = (fullscreen = false) => {
      if (!webUrl) return;
      const target = new URL(webUrl);
      target.searchParams.set("autoconnect", "true");
      target.searchParams.set("reconnect", "true");
      const features = fullscreen
        ? `noopener,noreferrer,width=${window.screen.availWidth},height=${window.screen.availHeight},left=0,top=0`
        : "noopener,noreferrer,width=1280,height=800";
      window.open(target.toString(), "_blank", features);
    };
    const CopyField = ({ label, value, password: isPassword = false }: { label: string; value: string; password?: boolean }) => (
      <div style={{ marginBottom: 12 }}>
        <Text type="secondary">{label}</Text>
        {isPassword ? (
          <Input.Password
            value={value}
            readOnly
            style={{ marginTop: 4 }}
            suffix={<Button size="small" icon={<CopyOutlined />} onClick={() => copyText(value, label)} />}
          />
        ) : (
          <Input
            value={value}
            readOnly
            style={{ marginTop: 4 }}
            suffix={<Button size="small" icon={<CopyOutlined />} onClick={() => copyText(value, label)} />}
          />
        )}
      </div>
    );

    return (
      <Tabs defaultActiveKey="client">
        <TabPane
          tab={<span><LinkOutlined /> VNC 客户端连接</span>}
          key="client"
        >
          <div style={{ padding: "12px 0" }}>
            {desktop.status === "error" && errorText && (
              <Alert
                type="error"
                showIcon
                message="桌面异常"
                description={errorText}
                style={{ marginBottom: 12 }}
              />
            )}
            <Space wrap style={{ marginBottom: 12 }}>
              <Tag color={statusColor(desktop.status)}>{statusText(desktop.status)}</Tag>
              <Tag color="blue">{desktop.protocol.toUpperCase()}</Tag>
              <Tag>{desktop.resolution}</Tag>
            </Space>
            <CopyField label="VNC URL" value={url} />
            <CopyField label="宿主机 IP" value={desktop.host_ip} />
            <CopyField label="端口" value={String(desktop.port || "")} />
            {password && (
              <CopyField label="密码" value={password} password />
            )}
            <p style={{ marginTop: 12, color: "#888" }}>
              请使用 VNC Viewer、TigerVNC 等客户端工具连接
            </p>
          </div>
        </TabPane>
        <TabPane
          tab={<span><GlobalOutlined /> 浏览器直接打开</span>}
          key="web"
        >
          <div style={{ padding: "12px 0" }}>
            {webUrl ? (
              <>
                <Space wrap style={{ marginBottom: 12 }}>
                  <Button type="primary" icon={<GlobalOutlined />} onClick={() => openWebDesktop(false)}>
                    打开
                  </Button>
                  <Button icon={<ReloadOutlined />} onClick={() => openWebDesktop(false)}>
                    重连
                  </Button>
                  <Button icon={<FullscreenOutlined />} onClick={() => openWebDesktop(true)}>
                    全屏窗口
                  </Button>
                  <Button icon={<CopyOutlined />} onClick={() => copyText(webUrl, "Web URL")}>
                    复制链接
                  </Button>
                </Space>
                <CopyField label="Web URL" value={webUrl} />
              </>
            ) : (
              <Empty description="该桌面不支持浏览器访问" />
            )}
          </div>
        </TabPane>
      </Tabs>
    );
  };

  return (
    <div className="rdp-page">
      <div className="rdp-page-header">
        <div>
          <h2 className="rdp-page-heading">{isAdmin ? "桌面管理" : "我的远程桌面"}</h2>
          <div className="rdp-page-description">
            延续参考图的控制台风格，统一桌面列表、协助卡片、状态标签和批量操作栏。
          </div>
        </div>
        <Space wrap>
          {isAdmin && <Tag color="red">管理员视图</Tag>}
          {isAdmin && (
            <Button
              danger
              icon={<CloseSquareOutlined />}
              loading={forceClosingAll}
              disabled={activeCount === 0}
              onClick={handleForceTerminateAll}
            >
              强制关闭全部
            </Button>
          )}
          <Button type="primary" icon={<PlusOutlined />} onClick={() => {
            fetchAvailableHosts();
            setModalOpen(true);
          }}>
            申请桌面
          </Button>
        </Space>
      </div>

      <div className="rdp-grid rdp-grid-4">
        <Card className="rdp-soft-card">
          <div className="rdp-stat-label">全部桌面</div>
          <div className="rdp-stat-value" style={{ fontSize: 32 }}>{desktops.length}</div>
          <div className="rdp-stat-meta"><span>当前登记的全部会话</span></div>
        </Card>
        <Card className="rdp-soft-card">
          <div className="rdp-stat-label">运行中</div>
          <div className="rdp-stat-value" style={{ fontSize: 32 }}>{runningCount}</div>
          <div className="rdp-stat-meta"><span>可直接连接或发起协助</span></div>
        </Card>
        <Card className="rdp-soft-card">
          <div className="rdp-stat-label">处理中</div>
          <div className="rdp-stat-value" style={{ fontSize: 32 }}>{pendingCount}</div>
          <div className="rdp-stat-meta"><span>正在启动或关闭的会话</span></div>
        </Card>
        <Card className="rdp-soft-card">
          <div className="rdp-stat-label">已关闭</div>
          <div className="rdp-stat-value" style={{ fontSize: 32 }}>{terminatedCount}</div>
          <div className="rdp-stat-meta"><span>可清理历史记录</span></div>
        </Card>
      </div>

      <Card className="rdp-table-card">
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 12, gap: 12, flexWrap: "wrap" }}>
          <div>
            <h3 className="rdp-section-title">我的桌面</h3>
            <div className="rdp-section-subtitle">按列表或卡片视图查看桌面会话，支持协助邀请和批量处理。</div>
          </div>
          <Tag color="blue">{desktops.length}</Tag>
        </div>

        {/* 批量操作栏 */}
        {selectedRowKeys.length > 0 && (
          <Card
            size="small"
            style={{ marginBottom: 12, background: "#f6ffed", borderColor: "#b7eb8f" }}
          >
            <Space>
              <CheckSquareOutlined style={{ color: "#52c41a" }} />
              <Text strong>已选择 {selectedRowKeys.length} 项</Text>
              <Button
                size="small"
                danger
                icon={<CloseSquareOutlined />}
                onClick={handleBatchTerminate}
              >
                批量关闭
              </Button>
              <Button
                size="small"
                danger
                ghost
                icon={<DeleteOutlined />}
                onClick={handleBatchDelete}
              >
                批量删除
              </Button>
              <Button size="small" onClick={() => setSelectedRowKeys([])}>
                取消选择
              </Button>
            </Space>
          </Card>
        )}

        {/* 视图切换 */}
        <div style={{ marginBottom: 12 }}>
          <Button.Group>
            <Button
              type={viewMode === "list" ? "primary" : "default"}
              icon={<UnorderedListOutlined />}
              onClick={() => setViewMode("list")}
            >
              列表
            </Button>
            <Button
              type={viewMode === "grid" ? "primary" : "default"}
              icon={<AppstoreOutlined />}
              onClick={() => setViewMode("grid")}
            >
              平铺
            </Button>
          </Button.Group>
        </div>

        {/* 列表视图 */}
        {viewMode === "list" && (
          <Card className="rdp-soft-card">
            <Table
              dataSource={desktops}
              columns={adminColumns as any}
              rowKey="id"
              loading={loading}
              rowSelection={rowSelection}
              locale={{ emptyText: <Empty description="暂无桌面会话" /> }}
              scroll={{ x: isAdmin ? 1400 : 1100 }}
            />
          </Card>
        )}

        {/* 平铺视图 */}
        {viewMode === "grid" && (
          <>
            {desktops.length === 0 ? (
              <Empty description="暂无桌面会话" style={{ marginTop: 40 }} />
            ) : (
              <Row gutter={[16, 16]}>
                {desktops.map((desktop) => (
                  <Col key={desktop.id} xs={24} sm={12} md={8} lg={6} xl={6}>
                    <DesktopCard
                      desktop={desktop}
                      selected={selectedRowKeys.includes(desktop.id)}
                      runtime={formatRuntime(desktop.created_at, desktop.status)}
                      sentInvites={mySentInvites}
                      onSelectedChange={(checked) => {
                        setSelectedRowKeys((prev) => {
                          if (checked) {
                            return prev.includes(desktop.id) ? prev : [...prev, desktop.id];
                          }
                          return prev.filter((key) => key !== desktop.id);
                        });
                      }}
                      onConnect={() => setConnectModal(desktop)}
                      onTerminate={() => handleTerminate(desktop.id)}
                      onInvite={() => {
                        setInviteDesktop(desktop);
                        setInviteModalOpen(true);
                      }}
                      onOpenFileTransfer={() => openFileTransfer(desktop.id, desktop.display_name || desktop.host_name || desktop.id)}
                      onStopInvite={async (inviteID) => {
                        try {
                          await collaborationAPI.stop(inviteID);
                          message.success("已终止协助");
                          fetchMySentInvites();
                        } catch (e: any) {
                          message.error(e.response?.data?.error || "终止失败");
                        }
                      }}
                      onDelete={() => handleDelete(desktop.id)}
                    />
                  </Col>
                ))}
              </Row>
            )}
          </>
        )}
      </Card>

      {/* ===== 模块二：他人桌面 ===== */}
      {invitedCollabs.length > 0 && (
        <Card className="rdp-table-card" style={{ marginTop: 4 }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 12, gap: 12, flexWrap: "wrap" }}>
            <div>
              <h3 className="rdp-section-title">他人桌面</h3>
              <div className="rdp-section-subtitle">处理收到的协助邀请，根据权限进入他人共享桌面。</div>
            </div>
            <Tag color="purple">{invitedCollabs.length}</Tag>
          </div>
          <Row gutter={[16, 16]}>
            {invitedCollabs.map((collab) => (
              <Col key={collab.id} xs={24} sm={12} md={8} lg={6} xl={4}>
                <Card
                  hoverable
                  className="rdp-soft-card"
                  style={{ position: "relative", overflow: "hidden" }}
                  bodyStyle={{ padding: "14px 14px 12px" }}
                >
                  <div style={cardShellStyle}>
                    {/* 左侧：图标 + 名称 + 状态 */}
                    <div style={{ flex: "0 1 120px", minWidth: 96, textAlign: "center" }}>
                      <div style={{ marginBottom: 4 }}>
                        <LinuxOutlined style={{ fontSize: 28, color: "#722ed1" }} />
                      </div>
                      <Text strong style={{ fontSize: 14, display: "block", overflowWrap: "anywhere" }}>
                        {collab.session?.host?.hostname || "共享桌面"}
                      </Text>
                      <Tag color="purple" style={{ marginTop: 6, fontSize: 10, height: "auto", lineHeight: 1.4 }}>
                        协助中
                      </Tag>
                      {collab.owner?.username && (
                        <div style={{ marginTop: 4 }}>
                          <Text type="secondary" style={{ fontSize: 11, overflowWrap: "anywhere" }}>{collab.owner?.username}</Text>
                        </div>
                      )}
                    </div>

                    {/* 中间：详细信息 */}
                    <div style={cardMetaStyle}>
                      <div><Text type="secondary">协议:</Text> {collab.session?.protocol?.toUpperCase() || "VNC"}</div>
                      <div><Text type="secondary">分辨率:</Text> {collab.session?.resolution || "1920x1080"}</div>
                      <div><Text type="secondary">端口:</Text> {collab.session?.port || "-"}</div>
                      <div><Text type="secondary">IP:</Text> <span style={{ overflowWrap: "anywhere" }}>{collab.session?.host?.ip || "-"}</span></div>
                    </div>

                    {/* 右侧：操作按钮 */}
                    <div style={cardActionStyle}>
                      <Space direction="vertical" style={{ width: "100%" }} size="small">
                        <div style={{ display: "flex", gap: 6 }}>
                          <Button
                            type="primary"
                            size="small"
                            icon={<PlayCircleOutlined />}
                            onClick={() => {
                              const directWebUrl = collab.session?.connection_info?.web_url;
                              const targetUrl = directWebUrl
                                ? new URL(directWebUrl)
                                : new URL(`${window.location.origin}/share/${collab.share_token}`);

                              if (!directWebUrl) {
                                targetUrl.searchParams.set("host", window.location.hostname);
                                targetUrl.searchParams.set("port", window.location.port || (window.location.protocol === "https:" ? "443" : "80"));
                                targetUrl.searchParams.set("path", `share/${collab.share_token}/websockify`);
                              }

                              targetUrl.searchParams.set("autoconnect", "true");
                              targetUrl.searchParams.set("reconnect", "true");

                              if (collab.session?.connection_info?.password) {
                                targetUrl.searchParams.set("password", collab.session.connection_info.password);
                              }
                              if (collab.role === "viewer") {
                                targetUrl.searchParams.set("view_only", "true");
                              }
                              window.open(targetUrl.toString(), "_blank", "noopener,noreferrer");
                            }}
                            title="协助访问"
                          />
                          <Button
                            danger
                            size="small"
                            icon={<LogoutOutlined />}
                            onClick={async () => {
                              try {
                                await collaborationAPI.stop(collab.id);
                                message.success("已退出协助");
                                fetchInvited();
                              } catch (e: any) {
                                message.error(e.response?.data?.error || "退出失败");
                              }
                            }}
                            title="退出协助"
                          />
                        </div>
                      </Space>
                    </div>
                  </div>
                </Card>
              </Col>
            ))}
          </Row>
        </Card>
      )}

      {/* 申请桌面弹窗 */}
      <Modal
        title="申请远程桌面"
        open={modalOpen}
        onCancel={() => setModalOpen(false)}
        onOk={() => form.submit()}
        confirmLoading={applying}
        okText="申请"
      >
        <Form form={form} layout="vertical" onFinish={handleApply}>
          <Form.Item
            name="display_name"
            label="桌面名称"
            rules={[{ max: 64, message: "桌面名称最多 64 个字符" }]}
          >
            <Input placeholder="例如：仿真任务、数据分析、临时调试" maxLength={64} />
          </Form.Item>
          <Form.Item
            name="purpose"
            label="用途说明"
            rules={[{ max: 200, message: "用途说明最多 200 个字符" }]}
          >
            <Input.TextArea
              placeholder="标注这个桌面的用途，方便后续区分和协同"
              maxLength={200}
              autoSize={{ minRows: 2, maxRows: 4 }}
              showCount
            />
          </Form.Item>
          <Form.Item name="host_id" label="运行节点" initialValue="auto" rules={[{ required: true }]}>
            <Select placeholder="请选择运行节点" optionLabelProp="label">
              <Select.Option value="auto" label="自动调度（推荐）">
                <Space direction="vertical" size={0}>
                  <Text strong>自动调度（推荐）</Text>
                  <Text type="secondary" style={{ fontSize: 12 }}>按节点负载和可用性自动选择</Text>
                </Space>
              </Select.Option>
              {availableHosts.map((host) => {
                const reason = hostDisabledReason(host);
                const loadText = `${host.current_sessions}/${host.max_sessions}`;
                return (
                  <Select.Option
                    key={host.id}
                    value={host.id}
                    label={`${host.hostname} · ${host.ip_address}`}
                    disabled={!!reason}
                  >
                    <Space direction="vertical" size={2} style={{ width: "100%" }}>
                      <Space wrap size={6}>
                        <Text strong>{host.hostname}</Text>
                        <Tag color={host.agent_managed ? "purple" : "default"}>{host.agent_managed ? "Agent" : "SSH"}</Tag>
                        <Tag color={reason ? "red" : "green"}>{reason ? "不可用" : "可用"}</Tag>
                      </Space>
                      <Text type="secondary" style={{ fontSize: 12 }}>
                        {host.ip_address} · {host.region || "默认区域"}/{host.az || "默认可用区"} · 负载 {loadText} · {host.cpu_cores || "-"}C/{formatMemory(host.total_ram_mb)}
                      </Text>
                      {reason && <Text type="danger" style={{ fontSize: 12 }}>{reason}</Text>}
                    </Space>
                  </Select.Option>
                );
              })}
            </Select>
          </Form.Item>
          <Form.Item name="desktop_env" label="桌面环境" initialValue="gnome" rules={[{ required: true }]}>
            <Select>
              <Select.Option value="gnome">GNOME</Select.Option>
              <Select.Option value="xfce">XFCE</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="performance_profile" label="性能档位" initialValue="balanced" rules={[{ required: true }]}>
            <Select
              onChange={(value) => {
                if (value === "low_bandwidth") {
                  form.setFieldsValue({ resolution: "1280x720", color_depth: 16, desktop_env: "xfce" });
                } else if (value === "quality") {
                  form.setFieldsValue({ resolution: "1920x1080", color_depth: 24 });
                }
              }}
            >
              <Select.Option value="quality">画质优先</Select.Option>
              <Select.Option value="balanced">均衡</Select.Option>
              <Select.Option value="low_bandwidth">低带宽</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="protocol" label="协议类型" initialValue="vnc" rules={[{ required: true }]}>
            <Select>
              <Select.Option value="vnc">VNC</Select.Option>
              <Select.Option value="rdp">RDP</Select.Option>
              <Select.Option value="spice">SPICE</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item noStyle shouldUpdate={(prev, curr) => prev.protocol !== curr.protocol}>
            {({ getFieldValue }) =>
              getFieldValue("protocol") === "vnc" ? (
                <Form.Item name="vnc_backend" label="VNC 后端" initialValue="turbovnc" rules={[{ required: true }]}>
                  <Select>
                    <Select.Option value="turbovnc">TurboVNC (推荐)</Select.Option>
                    <Select.Option value="tigervnc">TigerVNC</Select.Option>
                  </Select>
                </Form.Item>
              ) : null
            }
          </Form.Item>
          <Form.Item name="resolution" label="分辨率" initialValue="1920x1080" rules={[{ required: true }]}>
            <Select>
              <Select.Option value="1920x1080">1920x1080</Select.Option>
              <Select.Option value="1600x900">1600x900</Select.Option>
              <Select.Option value="1280x720">1280x720</Select.Option>
              <Select.Option value="1024x768">1024x768</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="color_depth" label="色深" initialValue={24}>
            <Select>
              <Select.Option value={24}>24-bit (真彩色)</Select.Option>
              <Select.Option value={16}>16-bit (高彩色)</Select.Option>
              <Select.Option value={8}>8-bit (256色)</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item noStyle shouldUpdate>
            {({ getFieldValue }) => {
              const profile = getFieldValue("performance_profile") || "balanced";
              const resolution = getFieldValue("resolution") || "";
              const colorDepth = getFieldValue("color_depth") || 24;
              const desktopEnv = getFieldValue("desktop_env") || "gnome";
              const selectedHost = availableHosts.find((host) => host.id === getFieldValue("host_id"));
              const reasons: string[] = [];
              if (profile === "quality") reasons.push("画质优先会提高持续带宽占用");
              if (resolution === "1920x1080") reasons.push("高分辨率会增加画面刷新数据量");
              if (colorDepth === 24) reasons.push("24-bit 色深比 16-bit 更耗带宽");
              if (desktopEnv === "gnome") reasons.push("GNOME 动画和合成效果可能带来额外刷新");
              if (selectedHost?.region && selectedHost.region !== "local") reasons.push("非本地区域节点可能受跨地域链路影响");
              if (highRecentBandwidth) reasons.push("近期已有会话带宽较高");
              if (profile === "low_bandwidth" || reasons.length === 0) return null;
              return (
                <Alert
                  type="warning"
                  showIcon
                  style={{ marginBottom: 12 }}
                  message="建议使用低带宽档位"
                  description={`${reasons.join("；")}。可以切换到低带宽档位，或手动选择 1280x720、16-bit、XFCE。`}
                />
              );
            }}
          </Form.Item>
        </Form>
      </Modal>

      {/* 连接弹窗 */}
      <Modal
        title="连接远程桌面"
        open={!!connectModal}
        onCancel={() => setConnectModal(null)}
        footer={null}
        width={560}
      >
        {connectModal && <ConnectModalContent desktop={connectModal} />}
      </Modal>

      {/* 邀请协助弹窗 */}
      <Modal
        title="邀请用户协助"
        open={inviteModalOpen}
        onCancel={() => {
          setInviteModalOpen(false);
          setInviteDesktop(null);
          inviteForm.resetFields();
        }}
        onOk={() => inviteForm.submit()}
        confirmLoading={inviteLoading}
        okText="发送邀请"
      >
        <Form
          form={inviteForm}
          layout="vertical"
          onFinish={async (values) => {
            if (!inviteDesktop) return;
            setInviteLoading(true);
            try {
              await collaborationAPI.create({
                session_id: inviteDesktop.id,
                invitee_id: values.invitee_username,  // 后端通过 username 查找用户
                role: values.role || "viewer",
              });
              message.success("邀请发送成功");
              setInviteModalOpen(false);
              setInviteDesktop(null);
              inviteForm.resetFields();
            } catch (e: any) {
              message.error(e.response?.data?.error || "邀请失败");
            } finally {
              setInviteLoading(false);
            }
          }}
        >
          <Form.Item label="桌面">
            <Text strong>{inviteDesktop?.host_name || ""}</Text>
            <div style={{ fontSize: 12, color: "#888" }}>
              {inviteDesktop?.resolution} | {inviteDesktop?.protocol?.toUpperCase()}
            </div>
          </Form.Item>
          <Form.Item
            name="invitee_username"
            label="被邀请用户名"
            rules={[{ required: true, message: "请输入被邀请用户的用户名" }]}
          >
            <Input placeholder="输入用户名" />
          </Form.Item>
          <Form.Item
            name="role"
            label="权限"
            initialValue="viewer"
            rules={[{ required: true }]}
          >
            <Select>
              <Select.Option value="viewer">仅观看</Select.Option>
              <Select.Option value="controller">可控制</Select.Option>
            </Select>
          </Form.Item>
        </Form>
      </Modal>
      <FileTransferModal />
      <FloatingTransferStatus />
    </div>
  );
};

export default DesktopsPage;
