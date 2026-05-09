import React from "react";
import ReactDOM from "react-dom/client";
import { ConfigProvider } from "antd";
import { BrowserRouter } from "react-router-dom";
import App from "./App";
import "./styles/theme.css";

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <ConfigProvider
      theme={{
        token: {
          colorPrimary: "#1677ff",
          colorInfo: "#1890ff",
          colorSuccess: "#52c41a",
          colorWarning: "#faad14",
          colorError: "#ff4d4f",
          colorText: "#1d2129",
          colorTextSecondary: "#4e5969",
          colorBgLayout: "#f5f7fa",
          colorBgContainer: "#ffffff",
          colorBorderSecondary: "#e5eaf2",
          borderRadius: 14,
          fontFamily:
            '"PingFang SC", "Microsoft YaHei", "Helvetica Neue", Arial, sans-serif',
        },
        components: {
          Layout: {
            bodyBg: "#f5f7fa",
            headerBg: "rgba(255,255,255,0.92)",
            siderBg: "#0b1426",
          },
          Card: {
            borderRadiusLG: 18,
            paddingLG: 20,
          },
          Button: {
            borderRadius: 10,
            fontWeight: 500,
            controlHeight: 38,
          },
          Input: {
            borderRadius: 10,
            controlHeight: 40,
          },
          Select: {
            borderRadius: 10,
            controlHeight: 40,
          },
          Table: {
            headerBg: "#f8fafc",
            headerColor: "#4e5969",
          },
          Modal: {
            borderRadiusLG: 18,
          },
          Tag: {
            borderRadiusSM: 999,
          },
        },
      }}
    >
      <BrowserRouter>
        <App />
      </BrowserRouter>
    </ConfigProvider>
  </React.StrictMode>,
);
