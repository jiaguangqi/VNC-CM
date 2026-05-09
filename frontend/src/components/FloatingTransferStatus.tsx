import React from "react";
import { Badge, Tooltip, Button, Progress } from "antd";
import {
  CloudUploadOutlined,
  CloudDownloadOutlined,
  CloseOutlined,
} from "@ant-design/icons";
import { useFileTransferStore } from "../stores/fileTransferStore";

const FloatingTransferStatus: React.FC = () => {
  const { visible, minimized, restore, close, tasks } =
    useFileTransferStore();

  // 如果没有任务且窗口已关闭，不显示
  if (tasks.length === 0 && !visible && !minimized) return null;

  const running = tasks.filter((t) => t.status === "running");
  const done = tasks.filter((t) => t.status === "done").length;
  const errors = tasks.filter((t) => t.status === "error").length;
  const activeCount = running.length;
  const totalCount = tasks.length;

  // 窗口展开时不需要显示浮动条
  if (visible && !minimized) return null;

  return (
    <div
      style={{
        position: "fixed",
        bottom: 20,
        right: 20,
        zIndex: 1100,
        display: "flex",
        flexDirection: "column",
        alignItems: "flex-end",
        gap: 8,
      }}
    >
      {/* 进度简介卡片 */}
      {activeCount > 0 && (
        <div
          style={{
            background: "#fff",
            borderRadius: 8,
            padding: "8px 12px",
            boxShadow: "0 2px 12px rgba(0,0,0,0.12)",
            border: "1px solid #e8e8e8",
            width: 220,
            cursor: "pointer",
          }}
          onClick={restore}
        >
          <div
            style={{
              fontSize: 12,
              color: "#666",
              marginBottom: 4,
              display: "flex",
              justifyContent: "space-between",
            }}
          >
            <span>
              <CloudUploadOutlined spin style={{ color: "#1890ff" }} /> {" "}
              {activeCount} 个任务进行中
            </span>
            <span style={{ color: "#999" }}>{done}/{totalCount}</span>
          </div>
          <Progress
            percent={Math.round(
              ((done + errors) / totalCount) * 100
            )}
            size="small"
            showInfo={false}
            status={errors > 0 ? "exception" : "active"}
          />
        </div>
      )}

      {/* 浮动按钮 */}
      <Tooltip
        title={
          activeCount > 0
            ? `文件传输中 (${activeCount})`
            : totalCount > 0
            ? `任务已完成 (${done}/${totalCount})`
            : "文件传输"
        }
      >
        <Badge
          count={activeCount > 0 ? activeCount : undefined}
          dot={done > 0 && activeCount === 0}
          color={activeCount > 0 ? "#1890ff" : "#52c41a"}
        >
          <Button
            type="primary"
            shape="circle"
            size="large"
            icon={
              activeCount > 0 ? (
                <CloudUploadOutlined />
              ) : totalCount > 0 ? (
                <CloudDownloadOutlined />
              ) : (
                <CloudUploadOutlined />
              )
            }
            onClick={restore}
          />
        </Badge>
      </Tooltip>
    </div>
  );
};

export default FloatingTransferStatus;
