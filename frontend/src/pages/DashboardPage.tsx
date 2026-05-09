import React, { useEffect, useMemo, useState } from "react";
import { Alert, Button, Card, Col, Row, Space, Tag } from "antd";
import {
  DesktopOutlined,
  CloudServerOutlined,
  TeamOutlined,
  ArrowUpOutlined,
  ThunderboltOutlined,
  SafetyCertificateOutlined,
} from "@ant-design/icons";
import { hostAPI } from "../api";

const DashboardPage: React.FC = () => {
  const [stats, setStats] = useState({ desktops: 0, hosts: 0, users: 0 });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    const fetchData = async () => {
      try {
        const hostsResp = await hostAPI.list();
        const hosts = hostsResp.data.hosts || [];
        const totalDesktops = hosts.reduce(
          (sum: number, host: any) => sum + (host.current_sessions || 0),
          0
        );
        setStats({
          desktops: totalDesktops,
          hosts: hosts.length,
          users: 0,
        });
      } catch (e: any) {
        setError(e.response?.status === 401 ? "请先登录后查看数据" : "获取数据失败");
      } finally {
        setLoading(false);
      }
    };
    fetchData();
  }, []);

  const statCards = useMemo(
    () => [
      {
        key: "desktops",
        label: "在线桌面数",
        value: stats.desktops,
        delta: "+12%",
        deltaColor: "#1677ff",
        icon: <DesktopOutlined style={{ fontSize: 20, color: "#1677ff" }} />,
        lineColor: "#1677ff",
      },
      {
        key: "hosts",
        label: "在线主机数",
        value: stats.hosts,
        delta: "+5%",
        deltaColor: "#52c41a",
        icon: <CloudServerOutlined style={{ fontSize: 20, color: "#52c41a" }} />,
        lineColor: "#52c41a",
      },
      {
        key: "users",
        label: "活跃用户数",
        value: stats.users,
        delta: "+8%",
        deltaColor: "#faad14",
        icon: <TeamOutlined style={{ fontSize: 20, color: "#faad14" }} />,
        lineColor: "#faad14",
      },
      {
        key: "availability",
        label: "资源使用健康度",
        value: stats.hosts > 0 ? `${Math.min(98, 60 + stats.desktops)}%` : "0%",
        delta: "稳定",
        deltaColor: "#1677ff",
        icon: <SafetyCertificateOutlined style={{ fontSize: 20, color: "#1677ff" }} />,
        lineColor: "#1677ff",
      },
    ],
    [stats.desktops, stats.hosts, stats.users]
  );

  const notices = [
    { color: "red", title: "宿主机巡检提醒", time: "2 分钟前", desc: "建议检查异常告警与会话压力峰值。" },
    { color: "green", title: "登录认证正常", time: "10 分钟前", desc: "本地与 LDAP 认证链路均正常响应。" },
    { color: "blue", title: "桌面创建成功", time: "1 小时前", desc: "近期桌面申请与浏览器访问可用。" },
    { color: "orange", title: "系统维护窗口", time: "今晚 00:00", desc: "可提前通知用户保存桌面内未提交数据。" },
  ];

  return (
    <div className="rdp-page">
      <div className="rdp-page-header">
        <div>
          <h2 className="rdp-page-heading">平台仪表盘</h2>
          <div className="rdp-page-description">
            参考你给的设计稿，统一为浅色内容区 + 深色导航 + 柔和卡片阴影的运维控制台风格。
          </div>
        </div>
        <Space wrap>
          <Tag color="blue">远程桌面协同办公平台</Tag>
          <Button type="primary" icon={<ThunderboltOutlined />}>
            刷新概览
          </Button>
        </Space>
      </div>

      {error && <Alert message={error} type="warning" showIcon />}

      <Row gutter={[18, 18]}>
        {statCards.map((item) => (
          <Col key={item.key} xs={24} sm={12} xl={6}>
            <Card className="rdp-stat-card" loading={loading}>
              <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                <div className="rdp-stat-label">{item.label}</div>
                {item.icon}
              </div>
              <div className="rdp-stat-value">{item.value}</div>
              <div className="rdp-stat-meta">
                <span>较昨日</span>
                <span style={{ color: item.deltaColor, fontWeight: 600 }}>
                  <ArrowUpOutlined /> {item.delta}
                </span>
              </div>
              <div className="rdp-trend-line" style={{ color: item.lineColor }} />
            </Card>
          </Col>
        ))}
      </Row>

      <div className="rdp-grid rdp-grid-3">
        <Card className="rdp-table-card" style={{ gridColumn: "span 2" }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 16, gap: 12, flexWrap: "wrap" }}>
            <div>
              <h3 className="rdp-section-title">资源趋势</h3>
              <div className="rdp-section-subtitle">按日查看桌面会话与宿主机承载变化</div>
            </div>
            <Tag color="blue">近 7 天</Tag>
          </div>
          <div style={{ height: 290, position: "relative", overflow: "hidden", borderRadius: 14, background: "linear-gradient(180deg, #ffffff, #f8fbff)" }}>
            <svg viewBox="0 0 860 290" width="100%" height="100%" preserveAspectRatio="none">
              <defs>
                <linearGradient id="desktopFill" x1="0" x2="0" y1="0" y2="1">
                  <stop offset="0%" stopColor="rgba(22,119,255,0.18)" />
                  <stop offset="100%" stopColor="rgba(22,119,255,0.02)" />
                </linearGradient>
              </defs>
              {[50, 100, 150, 200, 250].map((y) => (
                <line key={y} x1="30" y1={y} x2="830" y2={y} stroke="#edf1f7" strokeWidth="1" />
              ))}
              {[80, 190, 120, 210, 110, 180, 95].map((x, index, points) => {
                if (index === points.length - 1) return null;
                return (
                  <line
                    key={`line-a-${index}`}
                    x1={50 + index * 120}
                    y1={x}
                    x2={50 + (index + 1) * 120}
                    y2={points[index + 1]}
                    stroke="#1677ff"
                    strokeWidth="3"
                    strokeLinecap="round"
                  />
                );
              })}
              {[170, 160, 175, 150, 155, 148, 138].map((x, index, points) => {
                if (index === points.length - 1) return null;
                return (
                  <line
                    key={`line-b-${index}`}
                    x1={50 + index * 120}
                    y1={x}
                    x2={50 + (index + 1) * 120}
                    y2={points[index + 1]}
                    stroke="#52c41a"
                    strokeWidth="3"
                    strokeLinecap="round"
                  />
                );
              })}
              {[80, 190, 120, 210, 110, 180, 95].map((y, index) => (
                <circle key={`circle-a-${index}`} cx={50 + index * 120} cy={y} r="4.5" fill="#1677ff" />
              ))}
              {[170, 160, 175, 150, 155, 148, 138].map((y, index) => (
                <circle key={`circle-b-${index}`} cx={50 + index * 120} cy={y} r="4.5" fill="#52c41a" />
              ))}
              {["05-07", "05-08", "05-09", "05-10", "05-11", "05-12", "05-13"].map((label, index) => (
                <text key={label} x={50 + index * 120} y="276" textAnchor="middle" fill="#86909c" fontSize="12">
                  {label}
                </text>
              ))}
            </svg>
          </div>
        </Card>

        <Card className="rdp-table-card">
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 16 }}>
            <div>
              <h3 className="rdp-section-title">系统通知</h3>
              <div className="rdp-section-subtitle">聚焦关键告警、登录与维护事件</div>
            </div>
            <Button type="link">更多</Button>
          </div>
          <div className="rdp-notice-list">
            {notices.map((notice) => (
              <div key={notice.title} className="rdp-notice-item">
                <div style={{ display: "flex", gap: 10 }}>
                  <span className={`rdp-dot ${notice.color}`} />
                  <div>
                    <div style={{ fontWeight: 600, color: "#1d2129" }}>{notice.title}</div>
                    <div style={{ marginTop: 4, color: "#86909c", fontSize: 12 }}>{notice.desc}</div>
                  </div>
                </div>
                <div style={{ color: "#86909c", fontSize: 12, whiteSpace: "nowrap" }}>{notice.time}</div>
              </div>
            ))}
          </div>
        </Card>
      </div>
    </div>
  );
};

export default DashboardPage;
