import React, { useState, useEffect } from "react";
import {
  Card,
  Form,
  Input,
  Button,
  message,
  Row,
  Col,
  Divider,
  Typography,
  InputNumber,
  Switch,
  Tag,
  Space,
} from "antd";
import {
  SettingOutlined,
  DatabaseOutlined,
  LinkOutlined,
  SaveOutlined,
  ApiOutlined,
  SafetyCertificateOutlined,
} from "@ant-design/icons";
import { settingsAPI } from "../api";

const { Text } = Typography;

interface LDAPConfig {
  server1_url: string;
  server1_base_dn: string;
  server1_bind_dn: string;
  server1_bind_password: string;
  server2_url: string;
  server2_base_dn: string;
  server2_bind_dn: string;
  server2_bind_password: string;
  min_uid_number: number;
  enabled: boolean;
}

const SystemSettingsPage: React.FC = () => {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [testing, setTesting] = useState(false);
  const [enabled, setEnabled] = useState(false);

  useEffect(() => {
    fetchSettings();
  }, []);

  const fetchSettings = async () => {
    try {
      const res = await settingsAPI.getLDAP();
      if (res.data) {
        form.setFieldsValue(res.data);
        setEnabled(res.data.enabled || false);
      }
    } catch (e) {
      // ignore empty state
    }
  };

  const handleSave = async (values: LDAPConfig) => {
    setLoading(true);
    try {
      await settingsAPI.updateLDAP({ ...values, enabled });
      message.success("配置已保存");
    } catch (e: any) {
      message.error(e.response?.data?.error || "保存失败");
    } finally {
      setLoading(false);
    }
  };

  const handleTest = async () => {
    setTesting(true);
    try {
      const res = await settingsAPI.testLDAP();
      if (res.data?.success) {
        message.success("LDAP 连接测试成功");
      } else {
        message.warning(res.data?.message || "连接测试未通过");
      }
    } catch (e: any) {
      message.error(e.response?.data?.error || "连接测试失败");
    } finally {
      setTesting(false);
    }
  };

  return (
    <div className="rdp-page">
      <div className="rdp-page-header">
        <div>
          <h2 className="rdp-page-heading">系统设置</h2>
          <div className="rdp-page-description">
            采用与设计稿一致的浅色运维面板样式，突出配置层级、状态标签与操作按钮。
          </div>
        </div>
        <Space wrap>
          <Tag color="red">ADMIN ONLY</Tag>
          <Tag color={enabled ? "green" : "default"}>{enabled ? "LDAP 已启用" : "LDAP 未启用"}</Tag>
        </Space>
      </div>

      <Row gutter={[18, 18]} align="stretch" style={{ marginBottom: 18 }}>
        <Col xs={24} md={8} style={{ display: "flex" }}>
          <Card className="rdp-soft-card" style={{ flex: 1, height: "100%" }}>
            <Space align="start">
              <div className="rdp-brand-badge" style={{ width: 44, height: 44 }}>
                <SafetyCertificateOutlined />
              </div>
              <div>
                <div className="rdp-section-title">认证策略</div>
                <div className="rdp-section-subtitle">支持双 LDAP、高可用切换与最小 UID 过滤。</div>
              </div>
            </Space>
          </Card>
        </Col>
        <Col xs={24} md={8} style={{ display: "flex" }}>
          <Card className="rdp-soft-card" style={{ flex: 1, height: "100%" }}>
            <div className="rdp-stat-label">最小 UID Number</div>
            <div className="rdp-stat-value" style={{ fontSize: 32 }}>
              {form.getFieldValue("min_uid_number") || 500}
            </div>
            <div className="rdp-stat-meta">
              <span>仅允许不小于该值的用户登录</span>
            </div>
          </Card>
        </Col>
        <Col xs={24} md={8} style={{ display: "flex" }}>
          <Card className="rdp-soft-card" style={{ flex: 1, height: "100%" }}>
            <div className="rdp-stat-label">当前接入状态</div>
            <div className="rdp-stat-value" style={{ fontSize: 32 }}>
              {enabled ? "已接入" : "未接入"}
            </div>
            <div className="rdp-stat-meta">
              <span>建议保存后立即测试链路可用性</span>
            </div>
          </Card>
        </Col>
      </Row>

      <Card className="rdp-table-card">
        <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 18, gap: 12, flexWrap: "wrap" }}>
          <div>
            <h3 className="rdp-section-title">
              <SettingOutlined /> LDAP 对接配置
            </h3>
            <div className="rdp-section-subtitle">集中管理主备 LDAP 地址、Bind 账号和认证阈值。</div>
          </div>
          <Space>
            <Text type="secondary">启用 LDAP</Text>
            <Switch checked={enabled} onChange={setEnabled} />
          </Space>
        </div>

        <Form
          form={form}
          layout="vertical"
          onFinish={handleSave}
          initialValues={{ min_uid_number: 500 }}
        >
          <Row gutter={[18, 18]}>
            <Col xs={24} xl={12}>
              <Card
                type="inner"
                title={<><LinkOutlined /> LDAP Server 1</>}
                className="rdp-soft-card"
              >
                <Form.Item
                  name="server1_url"
                  label="服务器地址"
                  rules={[{ required: enabled, message: "请输入 LDAP 服务器地址" }]}
                >
                  <Input placeholder="ldap://172.22.2.252:389" />
                </Form.Item>
                <Form.Item
                  name="server1_base_dn"
                  label="Base DN"
                  rules={[{ required: enabled, message: "请输入 Base DN" }]}
                >
                  <Input placeholder="dc=seu,dc=com" />
                </Form.Item>
                <Form.Item
                  name="server1_bind_dn"
                  label="Bind DN"
                  rules={[{ required: enabled, message: "请输入 Bind DN" }]}
                >
                  <Input placeholder="cn=Manager,dc=seu,dc=com" />
                </Form.Item>
                <Form.Item
                  name="server1_bind_password"
                  label="Bind 密码"
                  rules={[{ required: enabled, message: "请输入密码" }]}
                >
                  <Input.Password placeholder="输入密码" />
                </Form.Item>
              </Card>
            </Col>
            <Col xs={24} xl={12}>
              <Card
                type="inner"
                title={<><LinkOutlined /> LDAP Server 2 (可选)</>}
                className="rdp-soft-card"
              >
                <Form.Item name="server2_url" label="服务器地址">
                  <Input placeholder="ldap://host:389" />
                </Form.Item>
                <Form.Item name="server2_base_dn" label="Base DN">
                  <Input placeholder="ou=People,dc=example,dc=com" />
                </Form.Item>
                <Form.Item name="server2_bind_dn" label="Bind DN">
                  <Input placeholder="可用 {username} 模板" />
                </Form.Item>
                <Form.Item name="server2_bind_password" label="Bind 密码">
                  <Input.Password placeholder="输入密码" />
                </Form.Item>
              </Card>
            </Col>
          </Row>

          <Divider />

          <Row gutter={[18, 18]} align="middle">
            <Col xs={24} md={8} xl={6}>
              <Form.Item
                name="min_uid_number"
                label="最小 UID Number"
                tooltip="仅 uidNumber 大于等于此值的用户允许登录"
              >
                <InputNumber min={0} max={65535} style={{ width: "100%" }} />
              </Form.Item>
            </Col>
            <Col xs={24} md={16} xl={18} style={{ textAlign: "right" }}>
              <Space size="middle" wrap>
                <Button icon={<ApiOutlined />} onClick={handleTest} loading={testing} disabled={!enabled}>
                  测试连接
                </Button>
                <Button type="primary" icon={<SaveOutlined />} htmlType="submit" loading={loading}>
                  保存配置
                </Button>
              </Space>
            </Col>
          </Row>
        </Form>

        <Divider />
        <Text type="secondary">
          支持双 LDAP、高可用一致性校验；仅 uidNumber 大于等于 {form.getFieldValue("min_uid_number") || 500} 的用户允许登录。
        </Text>
      </Card>
    </div>
  );
};

export default SystemSettingsPage;
