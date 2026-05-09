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
} from "antd";
import {
  DesktopOutlined,
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
} from "@ant-design/icons";
import { desktopAPI, collaborationAPI } from "../api";
import FileTransferModal from "../components/FileTransferModal";
import FloatingTransferStatus from "../components/FloatingTransferStatus";
import { useFileTransferStore } from "../stores/fileTransferStore";
import { useAuthStore } from "../stores/authStore";

const { Title, Text } = Typography;
const { TabPane } = Tabs;

interface DesktopSession {
  id: string;
  protocol: string;
  resolution: string;
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
  };
  created_at: string;
}

type ViewMode = "list" | "grid";

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
  const [inviteModalOpen, setInviteModalOpen] = useState(false);
  const [inviteForm] = Form.useForm();
  const [inviteLoading, setInviteLoading] = useState(false);
  const [inviteDesktop, setInviteDesktop] = useState<DesktopSession | null>(null);
  
  const [form] = Form.useForm();
  const { user } = useAuthStore();
  const isAdmin = user?.role === "admin";
  const { open: openFileTransfer } = useFileTransferStore();

  const runningCount = desktops.filter((desktop) => desktop.status === "running").length;
  const pendingCount = desktops.filter((desktop) => desktop.status === "pending").length;
  const terminatedCount = desktops.filter((desktop) => desktop.status === "terminated").length;


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

  useEffect(() => {
    fetchDesktops();
    fetchInvited();
    fetchMySentInvites();
    const interval = setInterval(fetchDesktops, 30000);
    const inviteInterval = setInterval(fetchMySentInvites, 10000);
    return () => {
      clearInterval(interval);
      clearInterval(inviteInterval);
    };
  }, []);

  // 批量操作
  const handleBatchTerminate = () => {
    const ids = selectedRowKeys as string[];
    if (ids.length === 0) return;
    const runningIds = ids.filter((id) => {
      const d = desktops.find((x) => x.id === id);
      return d && d.status === "running";
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

  const handleApply = async (values: any) => {
    setApplying(true);
    try {
      const res = await desktopAPI.create({
        protocol: values.protocol,
        resolution: values.resolution,
        color_depth: values.color_depth || 24,
        desktop_env: values.desktop_env,
        vnc_backend: values.protocol === "vnc" ? (values.vnc_backend || "turbovnc") : undefined,
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
      case "pending": return "orange";
      case "terminated": return "default";
      default: return "red";
    }
  };

  const statusText = (s: string) => {
    switch (s) {
      case "running": return "运行中";
      case "pending": return "创建中";
      case "terminated": return "已关闭";
      default: return s;
    }
  };

  // 表格列定义
  const baseColumns = [
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
            />
          </Tooltip>
          <Tooltip title="文件传输">
            <Button
              size="small"
              icon={<InboxOutlined />}
              onClick={() => openFileTransfer(record.id, record.host_name || record.id)}
            />
          </Tooltip>
          <Tooltip title="关闭">
            <Button
              size="small"
              danger
              icon={<StopOutlined />}
              onClick={() => handleTerminate(record.id)}
              disabled={record.status === "terminated"}
            />
          </Tooltip>
          {record.status !== "terminated" && (
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

    return (
      <Tabs defaultActiveKey="client">
        <TabPane
          tab={<span><LinkOutlined /> VNC 客户端连接</span>}
          key="client"
        >
          <div style={{ padding: "12px 0" }}>
            <p><b>协议:</b> {desktop.protocol.toUpperCase()}</p>
            <p><b>地址:</b> {desktop.host_ip}:{desktop.port}</p>
            <p>
              <b>URL:</b>
              <Input value={url} readOnly style={{ marginTop: 4 }}
                suffix={
                  <Button size="small" onClick={() => { navigator.clipboard.writeText(url); message.success("已复制"); }}>
                    复制
                  </Button>
                }
              />
            </p>
            {password && (
              <p>
                <b>密码:</b>
                <Input.Password value={password} readOnly style={{ marginTop: 4 }}
                  suffix={
                    <Button size="small" onClick={() => { navigator.clipboard.writeText(password); message.success("已复制"); }}>
                      复制
                    </Button>
                  }
                />
              </p>
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
            <p>无需安装客户端，直接在浏览器中打开 VNC 桌面：</p>
            {webUrl ? (
              <>
                <Button
                  type="primary"
                  icon={<GlobalOutlined />}
                  onClick={() => window.open(webUrl, "_blank", "width=1280,height=800")}
                  style={{ marginBottom: 12 }}
                >
                  新窗口打开 VNC 桌面
                </Button>
                <p style={{ marginTop: 8, color: "#888", fontSize: 12 }}>
                  如遇安全证书警告，请点击「高级」→「继续访问」
                </p>
                <Input.TextArea
                  value={webUrl}
                  readOnly
                  rows={2}
                  style={{ marginTop: 8 }}
                />
              </>
            ) : (
              <Empty description="该桌面不支持浏览器访问" />
            )}
          </div>
        </TabPane>
      </Tabs>
    );
  };

  // 桌面卡片组件（横向布局）
  const DesktopCard = ({ desktop }: { desktop: DesktopSession }) => {
    const isRunning = desktop.status === "running";
    const isTerminated = desktop.status === "terminated";
    const isWindows = desktop.host_os?.toLowerCase().includes("win");

    return (
      <Card
        hoverable
        className="rdp-soft-card"
        style={{
          position: "relative",
          overflow: "hidden",
        }}
        bodyStyle={{ padding: "14px 14px 12px" }}
      >
        <div style={cardShellStyle}>
          {/* checkbox far left */}
          <div style={{ flex: "0 0 auto", alignSelf: "center", paddingRight: 4 }}>
            <Checkbox
              checked={selectedRowKeys.includes(desktop.id)}
              onChange={(e) => {
                if (e.target.checked) {
                  setSelectedRowKeys([...selectedRowKeys, desktop.id]);
                } else {
                  setSelectedRowKeys(selectedRowKeys.filter((k) => k !== desktop.id));
                }
              }}
            />
          </div>

          {/* 左侧：图标 + 名称 + 状态 + 用户名 */}
          <div style={{ flex: "0 1 120px", minWidth: 96, textAlign: "center" }}>
            <div style={{ marginBottom: 4 }}>
              {isWindows ? (
                <WindowsOutlined style={{ fontSize: 28, color: "#1890ff" }} />
              ) : (
                <LinuxOutlined style={{ fontSize: 28, color: "#52c41a" }} />
              )}
            </div>
            <Text strong style={{ fontSize: 14, display: "block", overflowWrap: "anywhere" }}>
              {desktop.host_name}
            </Text>
            <Tag color={statusColor(desktop.status)} style={{ marginTop: 6, fontSize: 10, height: "auto", lineHeight: 1.4 }}>
              {statusText(desktop.status)}
            </Tag>
            {desktop.username && (
              <div style={{ marginTop: 4 }}>
                <Text type="secondary" style={{ fontSize: 11, overflowWrap: "anywhere" }}>{desktop.username}</Text>
              </div>
            )}
          </div>

          {/* 中间：详细信息 */}
          <div style={cardMetaStyle}>
            <div><Text type="secondary">协议:</Text> {desktop.protocol.toUpperCase()}</div>
            <div><Text type="secondary">分辨率:</Text> {desktop.resolution}</div>
            <div><Text type="secondary">端口:</Text> {desktop.port}</div>
            <div><Text type="secondary">IP:</Text> <span style={{ overflowWrap: "anywhere" }}>{desktop.host_ip}</span></div>
          </div>

          {/* 右侧：操作按钮 */}
          <div style={cardActionStyle}>
            <Space direction="vertical" style={{ width: "100%" }} size="small">
              <div style={{ display: "flex", gap: 6 }}>
                <Button
                  type="primary"
                  size="small"
                  icon={<PlayCircleOutlined />}
                  onClick={() => setConnectModal(desktop)}
                  disabled={!isRunning}
                  title="连接"
                />
                <Button
                  danger
                  size="small"
                  icon={<StopOutlined />}
                  onClick={() => handleTerminate(desktop.id)}
                  disabled={!isRunning}
                  title="关闭"
                />
                {isRunning && (
                  <Button
                    size="small"
                    icon={<TeamOutlined />}
                    onClick={() => {
                      setInviteDesktop(desktop);
                      setInviteModalOpen(true);
                    }}
                    title="邀请协助"
                  />
                )}
                {isRunning && (
                  <Button
                    size="small"
                    icon={<InboxOutlined />}
                    onClick={() => openFileTransfer(desktop.id, desktop.host_name || desktop.id)}
                    title="文件传输"
                  />
                )}
              </div>
              {/* 活跃协助管理 */}
              {mySentInvites.filter(inv => inv.session_id === desktop.id && inv.status === "active").length > 0 && (
                <div style={{ marginTop: 4, padding: "6px 8px", background: "#f0f5ff", borderRadius: 6, border: "1px solid #d6e4ff", width: "100%" }}>
                  <div style={{ fontSize: 9, color: "#1677ff", marginBottom: 4, fontWeight: 500 }}>活跃协助</div>
                  {mySentInvites
                    .filter(inv => inv.session_id === desktop.id && inv.status === "active")
                    .map(inv => (
                      <div key={inv.id} style={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 8, marginTop: 3 }}>
                        <span style={{ fontSize: 10 }}>{inv.invitee?.username || "未知用户"}</span>
                        <Button
                          size="small"
                          danger
                          style={{ fontSize: 11, padding: "0 6px", height: 22 }}
                          onClick={async () => {
                            try {
                              await collaborationAPI.stop(inv.id);
                              message.success("已终止协助");
                              fetchMySentInvites();
                            } catch (e: any) {
                              message.error(e.response?.data?.error || "终止失败");
                            }
                          }}
                        >
                          终止
                        </Button>
                      </div>
                    ))
                  }
                </div>
              )}
              {isTerminated && (
                <Button
                  danger
                  ghost
                  size="small"
                  icon={<DeleteOutlined />}
                  block
                  onClick={() => handleDelete(desktop.id)}
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
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setModalOpen(true)}>
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
          <div className="rdp-stat-label">创建中</div>
          <div className="rdp-stat-value" style={{ fontSize: 32 }}>{pendingCount}</div>
          <div className="rdp-stat-meta"><span>等待桌面服务初始化</span></div>
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
                  <Col key={desktop.id} xs={24} sm={12} md={8} lg={6} xl={4}>
                    <DesktopCard desktop={desktop} />
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
          <Form.Item name="desktop_env" label="桌面环境" initialValue="gnome" rules={[{ required: true }]}>
            <Select>
              <Select.Option value="gnome">GNOME</Select.Option>
              <Select.Option value="xfce">XFCE</Select.Option>
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
