import React, { useEffect, useMemo, useState } from "react";
import { Alert, Button, Card, Col, Row, Space, Tag } from "antd";
import { DesktopOutlined, CloudServerOutlined, TeamOutlined, ArrowUpOutlined, ArrowDownOutlined, ThunderboltOutlined, SafetyCertificateOutlined } from "@ant-design/icons";
import { statsAPI } from "../api";

function calcDelta(now: number, prev: number): { text: string; isUp: boolean } {
  if (prev === 0) return { text: "+0%", isUp: true };
  const delta = ((now - prev) / prev) * 100;
  const sign = delta >= 0 ? "+" : "";
  return { text: sign + delta.toFixed(1) + "%", isUp: delta >= 0 };
}

export default function DashboardPage() {
  const [stats, setStats] = useState({ desktops: 0, hosts: 0, users: 0 });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [trendData, setTrendData] = useState<any[]>([]);

  useEffect(() => {
    const fetchData = async () => {
      setLoading(true);
      try {
        const [overviewRes, trendRes] = await Promise.all([
          statsAPI.overview(),
          statsAPI.trend(7),
        ]);
        const overview = overviewRes.data?.data || overviewRes.data;
        const trend = trendRes.data?.data || trendRes.data;
        if (overview) {
          setStats({
            desktops: overview.total_desktops || 0,
            hosts: overview.total_hosts || 0,
            users: overview.active_users || 0,
          });
        }
        if (trend?.data) {
          setTrendData(trend.data);
        }
      } catch (e: any) {
        setError(e.response?.status === 401 ? "请先登录后查看数据" : "获取数据失败");
      } finally {
        setLoading(false);
      }
    };
    fetchData();
  }, []);

  const statCards = useMemo(() => {
    const desktopDelta = calcDelta(stats.desktops, 0);
    const hostDelta = calcDelta(stats.hosts, 0);
    const userDelta = calcDelta(stats.users, 0);
    const healthVal = stats.hosts > 0 ? Math.min(98, 60 + Math.round((stats.desktops / Math.max(1, stats.hosts)) * 10)) : 0;
    return [
      { key: "desktops", label: "在线桌面数", value: stats.desktops, delta: desktopDelta.text, isUp: desktopDelta.isUp, color: "#1677ff", icon: <DesktopOutlined style={{fontSize:20,color:"#1677ff"}}/> },
      { key: "hosts", label: "在线主机数", value: stats.hosts, delta: hostDelta.text, isUp: hostDelta.isUp, color: "#52c41a", icon: <CloudServerOutlined style={{fontSize:20,color:"#52c41a"}}/> },
      { key: "users", label: "活跃用户数(7天)", value: stats.users, delta: userDelta.text, isUp: userDelta.isUp, color: "#faad14", icon: <TeamOutlined style={{fontSize:20,color:"#faad14"}}/> },
      { key: "health", label: "资源使用健康度", value: healthVal + "%", delta: healthVal >= 80 ? "健康" : healthVal >= 50 ? "一般" : "紧张", isUp: true, color: healthVal >= 80 ? "#1677ff" : healthVal >= 50 ? "#faad14" : "#ff4d4f", icon: <SafetyCertificateOutlined style={{fontSize:20,color:healthVal >= 80 ? "#1677ff" : healthVal >= 50 ? "#faad14" : "#ff4d4f"}}/> },
    ];
  }, [stats]);

  // Trend chart SVG - fixed Y-axis scale
  const trendChart = useMemo(() => {
    if (!trendData.length) return null;
    const pts = trendData;
    const w = 860, h = 290, pl = 40, pr = 30, cw = w - pl - pr, ch = 220;

    const maxVal = Math.max(...pts.map((p: any) => Math.max(p.desktop_count || 0, p.host_count || 0, p.active_user_count || 0)), 1);
    const xStep = cw / (pts.length - 1 || 1);

    // 计算 Y 轴刻度：根据最大值动态决定步长，避免重复
    const maxLabel = Math.max(maxVal, 1);
    const tickStep = Math.ceil(maxLabel / 5); // 最多 5-6 个刻度
    const maxCeil = Math.ceil(maxLabel / tickStep) * tickStep;
    const getY = (v: number) => ch - (v / maxCeil) * ch * 0.8 + 10;

    const dp = pts.map((p: any, i: number) => ({ x: pl + i * xStep, y: getY(p.desktop_count || 0) }));
    const hp = pts.map((p: any, i: number) => ({ x: pl + i * xStep, y: getY(p.host_count || 0) }));

    const makeLines = (pts2: any[], color: string) => {
      const lines: any[] = [], circles: any[] = [];
      for (let i = 0; i < pts2.length - 1; i++) {
        lines.push(<line key={"l"+color+i} x1={pts2[i].x} y1={pts2[i].y} x2={pts2[i+1].x} y2={pts2[i+1].y} stroke={color} strokeWidth="3" strokeLinecap="round"/>);
      }
      for (let i = 0; i < pts2.length; i++) {
        circles.push(<circle key={"c"+color+i} cx={pts2[i].x} cy={pts2[i].y} r="4.5" fill={color}/>);
      }
      return { lines, circles };
    };
    const d = makeLines(dp, "#1677ff");
    const he = makeLines(hp, "#52c41a");

    // 动态 Y 轴网格线和标签
    const tickCount = Math.ceil(maxCeil / tickStep);
    const gridLines: any[] = [];
    const yLabels: any[] = [];
    for (let t = 0; t <= tickCount; t++) {
      const val = t * tickStep;
      if (val > maxCeil) break;
      const y = getY(val);
      gridLines.push(<line key={"g"+t} x1={pl} y1={y} x2={w-pr} y2={y} stroke="#edf1f7" strokeWidth="1"/>);
      yLabels.push(<text key={"yl"+t} x={pl - 8} y={y + 4} textAnchor="end" fill="#86909c" fontSize="11">{val}</text>);
    }

    const xLabels = pts.map((p: any, i: number) => (
      <text key={"xl"+i} x={pl + i * xStep} y={ch + 40} textAnchor="middle" fill="#86909c" fontSize="12">{(p.date || "").slice(5)}</text>
    ));

    return (
      <svg viewBox={"0 0 " + w + " " + h} width="100%" height="100%" preserveAspectRatio="none">
        <defs>
          <linearGradient id="dfill" x1="0" x2="0" y1="0" y2="1">
            <stop offset="0%" stopColor="rgba(22,119,255,0.18)"/>
            <stop offset="100%" stopColor="rgba(22,119,255,0.02)"/>
          </linearGradient>
        </defs>
        {gridLines}{yLabels}
        <polygon
          points={[...dp.map((p: any) => p.x + "," + p.y), (dp[dp.length-1]?.x || 0) + "," + (ch+20), (dp[0]?.x || 0) + "," + (ch+20)].join(" ")}
          fill="url(#dfill)"
        />
        {d.lines}{d.circles}{he.lines}{he.circles}{xLabels}
      </svg>
    );
  }, [trendData]);

  const notices = useMemo(() => {
    const items: any[] = [];
    if (trendData.length > 0) {
      const today = trendData[trendData.length - 1];
      const yesterday = trendData.length > 1 ? trendData[trendData.length - 2] : null;
      items.push({ color: today?.desktop_count > 0 ? "green" : "red", title: "桌面会话统计", time: "今日", desc: "今日桌面 " + (today?.desktop_count || 0) + " 个" + (yesterday ? "，昨日 " + yesterday.desktop_count + " 个" : "") });
      items.push({ color: today?.active_user_count > 0 ? "green" : "orange", title: "用户活跃概览", time: "今日", desc: "今日活跃用户 " + (today?.active_user_count || 0) + " 人" + (yesterday ? "，昨日 " + yesterday.active_user_count + " 人" : "") });
    }
    items.push({ color: "orange", title: "系统维护窗口", time: "每晚 00:00", desc: "建议提前保存桌面内未提交数据。" });
    return items;
  }, [trendData]);

  return (
    <div className="rdp-page">
      <div className="rdp-page-header">
        <div>
          <h2 className="rdp-page-heading">平台仪表盘</h2>
          <div className="rdp-page-description">实时监控远程桌面会话、宿主机健康与资源使用趋势。</div>
        </div>
        <Space wrap>
          <Tag color="blue">远程桌面协同办公平台</Tag>
          <Button type="primary" icon={<ThunderboltOutlined />} onClick={() => window.location.reload()}>刷新概览</Button>
        </Space>
      </div>

      {error && <Alert message={error} type="warning" showIcon style={{marginBottom:16}} />}

      <Row gutter={[18,18]}>
        {statCards.map(item => (
          <Col key={item.key} xs={24} sm={12} xl={6}>
            <Card className="rdp-stat-card" loading={loading}>
              <div style={{display:"flex",justifyContent:"space-between",alignItems:"center"}}>
                <div className="rdp-stat-label">{item.label}</div>
                {item.icon}
              </div>
              <div className="rdp-stat-value">{item.value}</div>
              <div className="rdp-stat-meta">
                <span>较上周</span>
                <span style={{color:item.color,fontWeight:600}}>
                  {item.isUp ? <ArrowUpOutlined/> : <ArrowDownOutlined/>} {item.delta}
                </span>
              </div>
              <div className="rdp-trend-line" style={{color:item.color}}/>
            </Card>
          </Col>
        ))}
      </Row>

      <div className="rdp-grid rdp-grid-3">
        <Card className="rdp-table-card" style={{gridColumn:"span 2"}}>
          <div style={{display:"flex",justifyContent:"space-between",alignItems:"center",marginBottom:16,gap:12,flexWrap:"wrap"}}>
            <div>
              <h3 className="rdp-section-title">资源趋势</h3>
              <div className="rdp-section-subtitle">按日查看桌面会话与宿主机承载变化</div>
            </div>
            <Space>
              <Tag color="blue">桌面</Tag>
              <Tag color="green">主机</Tag>
              <Tag color="blue">近7天</Tag>
            </Space>
          </div>
          <div style={{height:290,position:"relative",overflow:"hidden",borderRadius:14,background:"linear-gradient(180deg, #ffffff, #f8fbff)"}}>
            {trendChart || <div style={{display:"flex",alignItems:"center",justifyContent:"center",height:"100%",color:"#86909c"}}>暂无趋势数据</div>}
          </div>
        </Card>

        <Card className="rdp-table-card">
          <div style={{display:"flex",justifyContent:"space-between",alignItems:"center",marginBottom:16}}>
            <div>
              <h3 className="rdp-section-title">系统通知</h3>
              <div className="rdp-section-subtitle">聚焦关键告警、会话与维护事件</div>
            </div>
            <Button type="link">更多</Button>
          </div>
          <div className="rdp-notice-list">
            {notices.map((notice: any, idx: number) => (
              <div key={idx} className="rdp-notice-item">
                <div style={{display:"flex",gap:10}}>
                  <span className={`rdp-dot ${notice.color}`}/>
                  <div>
                    <div style={{fontWeight:600,color:"#1d2129"}}>{notice.title}</div>
                    <div style={{marginTop:4,color:"#86909c",fontSize:12}}>{notice.desc}</div>
                  </div>
                </div>
                <div style={{color:"#86909c",fontSize:12,whiteSpace:"nowrap"}}>{notice.time}</div>
              </div>
            ))}
          </div>
        </Card>
      </div>
    </div>
  );
}
