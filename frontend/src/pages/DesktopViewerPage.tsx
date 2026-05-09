import React from "react";
import { Button, Card, Space, Tag } from "antd";
import {
  FullscreenOutlined,
  CopyOutlined,
  DisconnectOutlined,
  TeamOutlined,
  DesktopOutlined,
  SafetyCertificateOutlined,
} from "@ant-design/icons";
import { useParams } from "react-router-dom";

const DesktopViewerPage: React.FC = () => {
  const { id } = useParams();

  return (
    <div className="rdp-page">
      <div className="rdp-page-header">
        <div>
          <h2 className="rdp-page-heading">远程桌面查看器</h2>
          <div className="rdp-page-description">
            当前会话 ID：{id || "-"}。工具栏和画面区域统一为深浅对比的控制台风格。
          </div>
        </div>
        <Space wrap>
          <Tag color="green">运行中</Tag>
          <Tag color="blue">VNC</Tag>
          <Tag>1920x1080</Tag>
        </Space>
      </div>

      <Card className="rdp-viewer-card">
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", gap: 12, flexWrap: "wrap", marginBottom: 18 }}>
          <Space wrap>
            <Button icon={<TeamOutlined />}>邀请协助</Button>
            <Button icon={<CopyOutlined />}>复制链接</Button>
            <Button icon={<FullscreenOutlined />}>全屏</Button>
            <Button danger icon={<DisconnectOutlined />}>断开</Button>
          </Space>
          <Space wrap>
            <Tag color="processing" icon={<DesktopOutlined />}>浏览器会话</Tag>
            <Tag icon={<SafetyCertificateOutlined />}>WebSocket 已就绪</Tag>
          </Space>
        </div>

        <Card className="rdp-soft-card rdp-desktop-preview" bodyStyle={{ minHeight: 560, display: "flex", alignItems: "center", justifyContent: "center" }}>
          <div style={{ textAlign: "center", color: "#fff", maxWidth: 520, padding: "24px 18px" }}>
            <div style={{ fontSize: 54, marginBottom: 12 }}>🖥️</div>
            <div style={{ fontSize: 24, fontWeight: 700 }}>远程桌面画面渲染区域</div>
            <div style={{ marginTop: 10, color: "rgba(255,255,255,0.72)", lineHeight: 1.8 }}>
              此处嵌入 noVNC / RDP Canvas 组件。外层容器已经切换为新的深色终端画布风格，和整体主题一致。
            </div>
            <div style={{ marginTop: 18, color: "rgba(255,255,255,0.5)", fontSize: 13 }}>
              WebSocket 连接：wss://gateway/.../{id || "session"}
            </div>
          </div>
        </Card>
      </Card>
    </div>
  );
};

export default DesktopViewerPage;
