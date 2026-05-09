import React, { useEffect, useMemo, useState } from "react";
import { Form, Input, Button, Checkbox, message, Typography } from "antd";
import {
  UserOutlined,
  LockOutlined,
  EyeInvisibleOutlined,
  EyeTwoTone,
} from "@ant-design/icons";
import { useNavigate } from "react-router-dom";
import { useAuthStore } from "../stores/authStore";
import { authAPI } from "../api";

const { Title, Text } = Typography;

type LoginMode = "local" | "ldap";

const LoginPage: React.FC = () => {
  const navigate = useNavigate();
  const { setToken, setUser } = useAuthStore();
  const [loading, setLoading] = useState(false);
  const [lastError, setLastError] = useState("");
  const [loginMode, setLoginMode] = useState<LoginMode>("local");
  const [viewportWidth, setViewportWidth] = useState<number>(
    typeof window === "undefined" ? 1440 : window.innerWidth
  );

  useEffect(() => {
    const onResize = () => setViewportWidth(window.innerWidth);
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, []);

  const isMobile = viewportWidth < 900;

  const styles = useMemo(
    () => ({
      page: {
        minHeight: "100vh",
        display: "flex",
        alignItems: "center",
        justifyContent: isMobile ? "center" : "flex-end",
        padding: isMobile ? "20px" : "3.2vw",
        backgroundImage:
          "linear-gradient(0deg, rgba(6, 18, 47, 0.12), rgba(6, 18, 47, 0.12)), url('/vnc-manager-img.png')",
        backgroundSize: "cover",
        backgroundPosition: isMobile ? "32% center" : "center center",
        backgroundRepeat: "no-repeat",
        overflow: "hidden",
      } as React.CSSProperties,
      panel: {
        width: "100%",
        maxWidth: isMobile ? 560 : 520,
        padding: isMobile ? "28px 22px 24px" : "44px 46px 36px",
        borderRadius: isMobile ? 22 : 28,
        background: "rgba(255,255,255,0.94)",
        border: "1px solid rgba(255,255,255,0.72)",
        boxShadow: "0 24px 70px rgba(7, 18, 40, 0.22)",
        backdropFilter: "blur(8px)",
      } as React.CSSProperties,
      title: {
        margin: 0,
        textAlign: "center" as const,
        fontSize: isMobile ? 26 : 34,
        lineHeight: 1.2,
        color: "#15284d",
      } as React.CSSProperties,
      subtitle: {
        display: "block",
        textAlign: "center" as const,
        marginTop: 10,
        marginBottom: isMobile ? 24 : 32,
        fontSize: isMobile ? 14 : 16,
        color: "#97a3bc",
      } as React.CSSProperties,
      input: {
        height: isMobile ? 54 : 58,
        borderRadius: 12,
        fontSize: 16,
      } as React.CSSProperties,
      options: {
        display: "flex",
        justifyContent: "space-between",
        alignItems: isMobile ? "flex-start" : "center",
        flexDirection: isMobile ? "column" : "row",
        gap: isMobile ? 10 : 16,
        marginTop: -4,
        marginBottom: 22,
      } as React.CSSProperties,
      primaryButton: {
        height: isMobile ? 54 : 58,
        borderRadius: 12,
        fontSize: isMobile ? 18 : 22,
        fontWeight: 700,
        boxShadow: "0 12px 28px rgba(34, 110, 255, 0.28)",
      } as React.CSSProperties,
      dividerRow: {
        display: "flex",
        alignItems: "center",
        gap: 14,
        margin: isMobile ? "24px 0 18px" : "28px 0 20px",
        color: "#a6b0c4",
        fontSize: isMobile ? 13 : 15,
        fontWeight: 600,
      } as React.CSSProperties,
      dividerLine: {
        flex: 1,
        height: 1,
        background: "rgba(205, 211, 224, 0.92)",
      } as React.CSSProperties,
      secondaryButton: {
        height: isMobile ? 54 : 58,
        borderRadius: 12,
        fontSize: isMobile ? 16 : 18,
        fontWeight: 600,
        color: "#6f7c97",
      } as React.CSSProperties,
      footer: {
        marginTop: 22,
        textAlign: "center" as const,
        color: "#9aa6be",
        fontSize: isMobile ? 13 : 14,
        lineHeight: 1.8,
      } as React.CSSProperties,
    }),
    [isMobile]
  );

  const handleLogin = async (values: any) => {
    setLoading(true);
    setLastError("");
    try {
      const resp = await authAPI.login(values.username, values.password);
      if (!resp.data || !resp.data.access_token) {
        throw new Error("后端未返回 access_token");
      }
      setToken(resp.data.access_token);
      setUser(resp.data.user);
      message.success("登录成功！正在跳转...");
      setTimeout(() => navigate("/"), 500);
    } catch (error: any) {
      const errMsg = error.response?.data?.error || error.message || "未知错误";
      setLastError(errMsg);
      message.error("登录失败: " + errMsg);
    } finally {
      setLoading(false);
    }
  };

  const actionText = loginMode === "ldap" ? "LDAP 用户登陆" : "登录";

  return (
    <div style={styles.page}>
      <div style={styles.panel}>
        <Title level={1} style={styles.title}>
          欢迎登录
        </Title>
        <Text style={styles.subtitle}>VNC 集群管理工具</Text>

        {lastError && (
          <div
            style={{
              background: "#fff2f0",
              border: "1px solid #ffccc7",
              color: "#cf1322",
              padding: "10px 12px",
              borderRadius: 12,
              marginBottom: 18,
              fontSize: 13,
            }}
          >
            错误详情: {lastError}
          </div>
        )}

        <Form onFinish={handleLogin} layout="vertical" initialValues={{ remember: true }}>
          <Form.Item
            name="username"
            rules={[
              {
                required: true,
                message: loginMode === "ldap" ? "请输入 LDAP 用户名" : "请输入用户名",
              },
            ]}
          >
            <Input
              prefix={<UserOutlined style={{ color: "#93a0b8", fontSize: 18 }} />}
              placeholder="用户名 / 邮箱"
              style={styles.input}
            />
          </Form.Item>
          <Form.Item name="password" rules={[{ required: true, message: "请输入密码" }]}>
            <Input.Password
              prefix={<LockOutlined style={{ color: "#93a0b8", fontSize: 18 }} />}
              placeholder="密码"
              iconRender={(visible) => (visible ? <EyeTwoTone /> : <EyeInvisibleOutlined />)}
              style={styles.input}
            />
          </Form.Item>

          <div style={styles.options}>
            <Checkbox defaultChecked>记住我</Checkbox>
            <Button type="link" style={{ padding: 0, fontSize: isMobile ? 14 : 16 }}>
              忘记密码?
            </Button>
          </div>

          <Form.Item style={{ marginBottom: 0 }}>
            <Button type="primary" htmlType="submit" loading={loading} block style={styles.primaryButton}>
              {actionText}
            </Button>
          </Form.Item>
        </Form>

        <div style={styles.dividerRow}>
          <div style={styles.dividerLine} />
          <span>其他登录方式</span>
          <div style={styles.dividerLine} />
        </div>

        <Button
          block
          style={styles.secondaryButton}
          onClick={() => setLoginMode((mode) => (mode === "local" ? "ldap" : "local"))}
        >
          {loginMode === "ldap" ? "本地用户登录" : "LDAP 用户登陆"}
        </Button>

        <div style={styles.footer}>
          还没有账号? <span style={{ color: "#2d7cff", fontWeight: 600 }}>联系管理员</span> 获取访问权限
        </div>
      </div>
    </div>
  );
};

export default LoginPage;
