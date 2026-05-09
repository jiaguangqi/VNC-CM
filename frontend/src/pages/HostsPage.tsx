import React, { useState, useEffect } from "react";
import { Table, Button, Tag, Space, Modal, Form, Input, message, Alert, Descriptions, Popconfirm, Card, Row, Col, Typography } from "antd";
import { PlusOutlined, EyeOutlined, DeleteOutlined, ToolOutlined, ExclamationCircleOutlined, MonitorOutlined, EditOutlined } from "@ant-design/icons";
import { hostAPI } from "../api";

const { Text } = Typography;

interface HostRecord {
  id: string;
  hostname: string;
  ip_address: string;
  os_type: string;
  max_sessions: number;
  current_sessions: number;
  status: string;
  ssh_username: string;
  ssh_port: number;
  cpu_cores: number;
  total_ram_mb: number;
  region: string;
  az: string;
  agent_version: string;
  last_heartbeat: string;
  labels: string[];
}

const HostsPage: React.FC = () => {
  const [hosts, setHosts] = useState<HostRecord[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [isEditModalOpen, setIsEditModalOpen] = useState(false);
  const [detailModalOpen, setDetailModalOpen] = useState(false);
  const [sshModalOpen, setSshModalOpen] = useState(false);
  const [currentHost, setCurrentHost] = useState<HostRecord | null>(null);

  const [form] = Form.useForm();
  const [editForm] = Form.useForm();

  const healthyHosts = hosts.filter((host) => host.status === "healthy").length;
  const maintenanceHosts = hosts.filter((host) => host.status === "maintenance").length;
  const fullHosts = hosts.filter((host) => host.status === "full").length;
  const totalSessions = hosts.reduce((sum, host) => sum + (host.current_sessions || 0), 0);

  const fetchHosts = async () => {
    setLoading(true);
    setError("");
    try {
      const resp = await hostAPI.list(undefined);
      setHosts(resp.data?.hosts || []);
    } catch (e: any) {
      const msg = e.response?.status === 401 ? "请先登录后查看数据"
        : e.response?.status === 403 ? "需要管理员权限"
        : "获取宿主机列表失败";
      setError(msg);
      setHosts([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchHosts();
  }, []);

  const handleDelete = async (host: HostRecord) => {
    if (host.current_sessions > 0) {
      message.error(`该宿主机上有 ${host.current_sessions} 个正在运行的桌面，无法删除`);
      return;
    }
    try {
      await hostAPI.delete(host.id);
      message.success(`宿主机 "${host.hostname}" 已删除`);
      fetchHosts();
    } catch (e: any) {
      message.error(e.response?.data?.error || "删除失败");
    }
  };

  const handleMaintenance = async (host: HostRecord) => {
    try {
      await hostAPI.update(host.id, { status: "maintenance" });
      message.success(
        <span>
          宿主机 <b>{host.hostname}</b> 已进入维护模式<br/>
          已通知该节点上 {host.current_sessions} 个桌面的用户尽快保存数据并关闭桌面
        </span>
      );
      fetchHosts();
    } catch (e: any) {
      message.error(e.response?.data?.error || "维护模式切换失败");
    }
  };

  const handleRestore = async (host: HostRecord) => {
    try {
      await hostAPI.update(host.id, { status: "healthy" });
      message.success(`宿主机 "${host.hostname}" 已恢复上线`);
      fetchHosts();
    } catch (e: any) {
      message.error(e.response?.data?.error || "恢复失败");
    }
  };

  const handleAddHost = async (values: any) => {
    try {
      await hostAPI.create(values);
      message.success("宿主机添加成功");
      setIsModalOpen(false);
      form.resetFields();
      fetchHosts();
    } catch (e: any) {
      message.error(e.response?.data?.error || "添加失败");
    }
  };

  const openWebSSH = async (host: HostRecord) => {
    setCurrentHost(host);
    setSshModalOpen(true);
  };

  const statusColorMap: Record<string, string> = {
    init: "default",
    healthy: "green",
    full: "orange",
    offline: "red",
    maintenance: "blue",
  };


  const handleEdit = async (values: any) => {
    if (!currentHost) return;
    try {
      await hostAPI.update(currentHost.id, { max_sessions: parseInt(values.max_sessions) });
      message.success(`宿主机 "${currentHost.hostname}" 最大会话数已更新为 ${values.max_sessions}`);
      setIsEditModalOpen(false);
      editForm.resetFields();
      fetchHosts();
    } catch (e: any) {
      message.error(e.response?.data?.error || "更新失败");
    }
  };

  const openEditModal = (host: HostRecord) => {
    setCurrentHost(host);
    editForm.setFieldsValue({
      hostname: host.hostname,
      ip_address: host.ip_address,
      os_type: host.os_type,
      max_sessions: host.max_sessions,
    });
    setIsEditModalOpen(true);
  };
  const columns = [
    { title: "主机名", dataIndex: "hostname", key: "hostname", width: 140 },
    { title: "IP 地址", dataIndex: "ip_address", key: "ip_address", width: 130 },
    {
      title: "OS",
      dataIndex: "os_type",
      key: "os_type",
      width: 80,
      render: (os: string) =>
        os === "linux" ? <Tag color="blue">Linux</Tag> : <Tag color="cyan">Windows</Tag>,
    },
    {
      title: "会话数",
      key: "sessions",
      width: 90,
      render: (_: any, r: HostRecord) => (
        <span style={{ color: r.current_sessions >= r.max_sessions ? "#ff4d4f" : "inherit" }}>
          {r.current_sessions || 0} / {r.max_sessions || 0}
        </span>
      ),
    },
    {
      title: "资源",
      key: "resources",
      width: 120,
      render: (_: any, r: HostRecord) => (
        <span style={{ fontSize: 12, color: "#666" }}>
          {r.cpu_cores || "-"}C / {r.total_ram_mb ? (r.total_ram_mb / 1024).toFixed(1) : "-"}G
        </span>
      ),
    },
    {
      title: "状态",
      dataIndex: "status",
      key: "status",
      width: 100,
      render: (status: string) => (
        <Tag color={statusColorMap[status] || "default"}>{status?.toUpperCase()}</Tag>
      ),
    },
    { title: "区域", dataIndex: "region", key: "region", width: 100 },
    {
      title: "心跳",
      key: "heartbeat",
      width: 100,
      render: (_: any, r: HostRecord) => {
        if (!r.last_heartbeat) return <span style={{ color: "#999" }}>-</span>;
        const diff = Date.now() - new Date(r.last_heartbeat).getTime();
        const mins = Math.floor(diff / 60000);
        if (mins < 1) return <Tag color="green">刚刚</Tag>;
        if (mins < 5) return <Tag color="orange">{mins}分前</Tag>;
        return <Tag color="red">{Math.floor(mins / 60)}小时前</Tag>;
      },
    },
    {
      title: "操作",
      key: "action",
      width: 280,
      render: (_: any, record: HostRecord) => (
        <Space>
          <Button icon={<MonitorOutlined />} size="small" onClick={() => openWebSSH(record)}>
            WebSSH
          </Button>
          <Button icon={<EyeOutlined />} size="small" onClick={() => { setCurrentHost(record); setDetailModalOpen(true); }}>
            详情
          </Button>
          <Button icon={<EditOutlined />} size="small" onClick={() => openEditModal(record)}>
            编辑
          </Button>
          {record.status === "maintenance" ? (
            <Button type="primary" size="small" onClick={() => handleRestore(record)}>
              恢复上线
            </Button>
          ) : (
            <Popconfirm
              title={`将 "${record.hostname}" 设为维护模式？`}
              description={
                record.current_sessions > 0
                  ? `该节点上有 ${record.current_sessions} 个正在运行的桌面，用户将收到维护通知`
                  : "该节点将进入维护模式，无法新建桌面"
              }
              onConfirm={() => handleMaintenance(record)}
              okText="确认"
              cancelText="取消"
              icon={<ExclamationCircleOutlined style={{ color: "#faad14" }} />}
            >
              <Button icon={<ToolOutlined />} size="small">
                维护
              </Button>
            </Popconfirm>
          )}
          <Popconfirm
            title={`删除宿主机 "${record.hostname}"？`}
            description={
              record.current_sessions > 0
                ? `❌ 该节点有 ${record.current_sessions} 个运行中的桌面，无法删除`
                : "此操作不可恢复"
            }
            onConfirm={() => handleDelete(record)}
            okText="删除"
            cancelText="取消"
            okButtonProps={{ danger: true, disabled: record.current_sessions > 0 }}
          >
            <Button danger icon={<DeleteOutlined />} size="small" />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div className="rdp-page">
      <div className="rdp-page-header">
        <div>
          <h2 className="rdp-page-heading">宿主机管理</h2>
          <div className="rdp-page-description">参考设计稿统一了宿主机概览、列表容器和操作按钮层级。</div>
        </div>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setIsModalOpen(true)}>
          添加宿主机
        </Button>
      </div>

      <Row gutter={[16, 16]}>
        <Col xs={24} md={12} xl={6}><Card className="rdp-soft-card"><div className="rdp-stat-label">宿主机总数</div><div className="rdp-stat-value" style={{ fontSize: 32 }}>{hosts.length}</div><div className="rdp-stat-meta"><span>平台当前已登记节点</span></div></Card></Col>
        <Col xs={24} md={12} xl={6}><Card className="rdp-soft-card"><div className="rdp-stat-label">健康节点</div><div className="rdp-stat-value" style={{ fontSize: 32 }}>{healthyHosts}</div><div className="rdp-stat-meta"><span>可承载新桌面的节点数</span></div></Card></Col>
        <Col xs={24} md={12} xl={6}><Card className="rdp-soft-card"><div className="rdp-stat-label">维护中</div><div className="rdp-stat-value" style={{ fontSize: 32 }}>{maintenanceHosts}</div><div className="rdp-stat-meta"><span>已切换到维护状态</span></div></Card></Col>
        <Col xs={24} md={12} xl={6}><Card className="rdp-soft-card"><div className="rdp-stat-label">当前会话负载</div><div className="rdp-stat-value" style={{ fontSize: 32 }}>{totalSessions}</div><div className="rdp-stat-meta"><span>{fullHosts > 0 ? `满载节点 ${fullHosts} 台` : "当前没有满载节点"}</span></div></Card></Col>
      </Row>

      {error && <Alert message={error} type="warning" showIcon />}

      <Card className="rdp-table-card">
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 12, gap: 12, flexWrap: "wrap" }}>
          <div>
            <h3 className="rdp-section-title">宿主机列表</h3>
            <div className="rdp-section-subtitle">查看节点状态、资源占用、心跳与维护操作。</div>
          </div>
          <Tag color="blue">{hosts.length} 台节点</Tag>
        </div>
        <Table
          columns={columns}
          dataSource={hosts}
          rowKey="id"
          loading={loading}
          scroll={{ x: 1100 }}
          size="middle"
        />
      </Card>

      {/* 添加宿主机模态框 */}
      <Modal
        title="添加宿主机"
        open={isModalOpen}
        onOk={() => form.submit()}
        onCancel={() => setIsModalOpen(false)}
        width={600}
        destroyOnClose
      >
        <Form form={form} onFinish={handleAddHost} layout="vertical">
          <Form.Item name="hostname" label="主机名" rules={[{ required: true }]}
            tooltip="用于标识该宿主机的唯一名称">
            <Input placeholder="如：gpu-node-01" />
          </Form.Item>
          <Form.Item name="ip_address" label="IP 地址" rules={[{ required: true }]}>
            <Input placeholder="10.0.1.10" />
          </Form.Item>
          <Form.Item name="os_type" label="操作系统" rules={[{ required: true }]}>
            <Input placeholder="linux 或 windows" />
          </Form.Item>
          <Form.Item name="max_sessions" label="最大并发桌面数" rules={[{ required: true }]}>
            <Input type="number" placeholder="10" />
          </Form.Item>
          <Form.Item name="ssh_username" label="SSH 用户名">
            <Input placeholder="root" />
          </Form.Item>
          <Form.Item name="ssh_password" label="SSH 密码">
            <Input.Password placeholder="添加时加密存储" />
          </Form.Item>
          <Form.Item name="ssh_port" label="SSH 端口" initialValue={22}>
            <Input type="number" />
          </Form.Item>
        </Form>
      </Modal>

      {/* 详情模态框 */}
      <Modal
        title={`宿主机详情 - ${currentHost?.hostname || ""}`}
        open={detailModalOpen}
        onCancel={() => setDetailModalOpen(false)}
        width={650}
        footer={[
          <Button key="close" onClick={() => setDetailModalOpen(false)}>关闭</Button>,
        ]}
      >
        {currentHost && (
          <Descriptions bordered column={2}>
            <Descriptions.Item label="主机名">{currentHost.hostname}</Descriptions.Item>
            <Descriptions.Item label="IP 地址">{currentHost.ip_address}</Descriptions.Item>
            <Descriptions.Item label="操作系统">{currentHost.os_type}</Descriptions.Item>
            <Descriptions.Item label="状态">
              <Tag color={statusColorMap[currentHost.status] || "default"}>
                {currentHost.status?.toUpperCase()}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item label="CPU 核心">{currentHost.cpu_cores || "-"}</Descriptions.Item>
            <Descriptions.Item label="内存 (GB)">{(currentHost.total_ram_mb ? (currentHost.total_ram_mb / 1024).toFixed(1) : "-")}</Descriptions.Item>
            <Descriptions.Item label="最大会话">{currentHost.max_sessions || 0}</Descriptions.Item>
            <Descriptions.Item label="当前会话">{currentHost.current_sessions || 0}</Descriptions.Item>
            <Descriptions.Item label="区域">{currentHost.region || "-"}</Descriptions.Item>
            <Descriptions.Item label="可用区">{currentHost.az || "-"}</Descriptions.Item>
            <Descriptions.Item label="SSH 用户名">{currentHost.ssh_username || "-"}</Descriptions.Item>
            <Descriptions.Item label="SSH 端口">{currentHost.ssh_port || 22}</Descriptions.Item>
            <Descriptions.Item label="Agent 版本">{currentHost.agent_version || "-"}</Descriptions.Item>
            <Descriptions.Item label="最后心跳">
              {currentHost.last_heartbeat
                ? new Date(currentHost.last_heartbeat).toLocaleString("zh-CN")
                : "-"}
            </Descriptions.Item>
            <Descriptions.Item label="标签" span={2}>
              {currentHost.labels?.length
                ? currentHost.labels.map((l) => <Tag key={l}>{l}</Tag>)
                : "无"}
            </Descriptions.Item>
          </Descriptions>
        )}
      </Modal>

      {/* WebSSH 模态框 */}
      <Modal
        title={`WebSSH - ${currentHost?.hostname || ""}`}
        open={sshModalOpen}
        onCancel={() => setSshModalOpen(false)}
        width={960}
        footer={[
          <Button key="close" onClick={() => setSshModalOpen(false)}>关闭</Button>,
        ]}
        bodyStyle={{ padding: 0, height: 520 }}
        destroyOnClose
      >
        <iframe
          src="http://10.10.38.148:7681"
          style={{ width: "100%", height: "100%", border: "none", background: "#1e1e1e" }}
          title="WebSSH Terminal"
        />
      </Modal>

      {/* 编辑宿主机模态框 */}
      <Modal
        title={`编辑宿主机 - ${currentHost?.hostname || ""}`}
        open={isEditModalOpen}
        onOk={() => editForm.submit()}
        onCancel={() => { setIsEditModalOpen(false); editForm.resetFields(); }}
        width={500}
        destroyOnClose
      >
        <Form form={editForm} onFinish={handleEdit} layout="vertical">
          <Form.Item label="主机名">
            <Input disabled value={currentHost?.hostname} />
          </Form.Item>
          <Form.Item label="IP 地址">
            <Input disabled value={currentHost?.ip_address} />
          </Form.Item>
          <Form.Item label="操作系统">
            <Input disabled value={currentHost?.os_type} />
          </Form.Item>
          <Form.Item
            name="max_sessions"
            label="最大并发桌面数"
            rules={[
              { required: true, message: "请输入最大会话数" },
              { type: "number", min: 0, message: "不能为负数", transform: (v: any) => Number(v) },
            ]}
          >
            <Input type="number" placeholder="如：10" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default HostsPage;
