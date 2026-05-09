import React, { useEffect, useMemo, useState } from "react";
import { Layout, Menu, Button, Space, Tag, Avatar, Tooltip } from "antd";
import { Link, useLocation, Outlet, useNavigate } from "react-router-dom";
import {
  DesktopOutlined,
  CloudServerOutlined,
  DashboardOutlined,
  LogoutOutlined,
  SettingOutlined,
  UserOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
} from "@ant-design/icons";
import { useAuthStore } from "../stores/authStore";

const { Sider, Header, Content } = Layout;

const AppLayout: React.FC = () => {
  const location = useLocation();
  const navigate = useNavigate();
  const { user, logout } = useAuthStore();
  const [collapsed, setCollapsed] = useState(false);

  useEffect(() => {
    if (user && user.role !== "admin" && location.pathname === "/dashboard") {
      navigate("/desktops", { replace: true });
    }
  }, [user, location.pathname, navigate]);

  const handleLogout = () => {
    logout();
    navigate("/login", { replace: true });
  };

  const isAdmin = user?.role === "admin";

  const menuItems = [
    ...(isAdmin
      ? [
          {
            key: "/dashboard",
            icon: <DashboardOutlined />,
            label: <Link to="/dashboard">仪表盘</Link>,
          },
          {
            key: "/hosts",
            icon: <CloudServerOutlined />,
            label: <Link to="/hosts">宿主机管理</Link>,
          },
        ]
      : []),
    {
      key: "/desktops",
      icon: <DesktopOutlined />,
      label: <Link to="/desktops">桌面管理</Link>,
    },
    ...(isAdmin
      ? [
          {
            key: "/settings",
            icon: <SettingOutlined />,
            label: <Link to="/settings">系统设置</Link>,
          },
        ]
      : []),
  ];

  const pageMeta = useMemo(() => {
    if (location.pathname.startsWith("/hosts")) {
      return {
        title: "宿主机管理",
        subtitle: "统一管理远程桌面承载节点、容量状态与维护操作。",
      };
    }
    if (location.pathname.startsWith("/settings")) {
      return {
        title: "系统设置",
        subtitle: "集中维护 LDAP、认证与平台接入参数。",
      };
    }
    if (location.pathname.startsWith("/desktops/")) {
      return {
        title: "远程桌面查看器",
        subtitle: "在浏览器内查看和操作远程桌面会话。",
      };
    }
    if (location.pathname.startsWith("/desktops")) {
      return {
        title: isAdmin ? "桌面管理" : "我的远程桌面",
        subtitle: isAdmin
          ? "查看平台桌面会话、协助邀请和访问状态。"
          : "管理自己的远程桌面、协助邀请和连接入口。",
      };
    }
    return {
      title: "远程桌面协同办公平台",
      subtitle: "高效管理、稳定连接、智能运维。",
    };
  }, [isAdmin, location.pathname]);

  return (
    <Layout className="rdp-app-shell">
      {isAdmin && (
        <Sider
          width={248}
          collapsedWidth={88}
          collapsible
          collapsed={collapsed}
          trigger={null}
        >
          <div className="rdp-sider-inner">
            <div className="rdp-brand">
              <div className="rdp-brand-badge">VNC</div>
              {!collapsed && (
                <>
                  <div className="rdp-brand-title">远程桌面协同办公平台</div>
                  <div className="rdp-brand-subtitle">
                    高效管理 · 稳定连接 · 智能运维
                  </div>
                </>
              )}
            </div>
            <Menu
              selectedKeys={[location.pathname]}
              mode="inline"
              items={menuItems}
            />
          </div>
        </Sider>
      )}
      <Layout className="rdp-main-layout">
        <Header className="rdp-topbar">
          <div
            style={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              gap: 16,
              flexWrap: "wrap",
            }}
          >
            <div style={{ display: "flex", alignItems: "center", gap: 14 }}>
              {isAdmin ? (
                <Button
                  icon={collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
                  onClick={() => setCollapsed((value) => !value)}
                />
              ) : (
                <div className="rdp-brand-badge" style={{ width: 38, height: 38 }}>
                  VNC
                </div>
              )}
              <div>
                <h1 className="rdp-topbar-title">{pageMeta.title}</h1>
                <div className="rdp-topbar-subtitle">{pageMeta.subtitle}</div>
              </div>
            </div>
            <Space size="middle" wrap>
              {user && (
                <>
                  <Tag
                    color={isAdmin ? "processing" : "blue"}
                    style={{ paddingInline: 10, height: 30, lineHeight: "28px" }}
                  >
                    {isAdmin ? "管理员" : "普通用户"}
                  </Tag>
                  <Space size="small">
                    <Avatar size={34} icon={<UserOutlined />} style={{ background: "#e8f2ff", color: "#1677ff" }} />
                    <div>
                      <div style={{ fontWeight: 600, color: "#1d2129", lineHeight: 1.2 }}>
                        {user.username}
                      </div>
                      <div style={{ fontSize: 12, color: "#86909c", marginTop: 2 }}>
                        {isAdmin ? "平台管理账户" : "桌面使用账户"}
                      </div>
                    </div>
                  </Space>
                  <Tooltip title="退出当前登录">
                    <Button icon={<LogoutOutlined />} onClick={handleLogout}>
                      退出
                    </Button>
                  </Tooltip>
                </>
              )}
            </Space>
          </div>
        </Header>
        <Content className="rdp-content-shell">
          <Outlet />
        </Content>
      </Layout>
    </Layout>
  );
};

export default AppLayout;
