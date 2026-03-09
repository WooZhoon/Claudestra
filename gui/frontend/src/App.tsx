import { useState, useCallback, useEffect } from 'react';
import { EventsOn } from '../wailsjs/runtime/runtime';
import Sidebar from './components/Sidebar';
import LogPanel from './components/LogPanel';
import InputBar from './components/InputBar';
import ReportPanel from './components/ReportPanel';
import ProjectSetup from './components/ProjectSetup';

import * as WailsApp from '../wailsjs/go/main/App';

interface AgentStatus {
  id: string;
  role: string;
  status: string;
  isConsumer: boolean;
}

export default function App() {
  const [projectOpen, setProjectOpen] = useState(false);
  const [agents, setAgents] = useState<AgentStatus[]>([]);
  const [selectedAgent, setSelectedAgent] = useState<string | null>(null);
  const [logs, setLogs] = useState<string[]>(['🎼 Claudestra GUI 시작. 프로젝트를 열거나 생성하세요.']);
  const [report, setReport] = useState('');
  const [showReport, setShowReport] = useState(false);
  const [running, setRunning] = useState(false);

  // 실시간 로그 수신
  useEffect(() => {
    EventsOn('log', (msg: string) => {
      setLogs(prev => [...prev, msg]);
    });
  }, []);

  const refreshStatuses = useCallback(async () => {
    try {
      const statuses = await WailsApp.GetAgentStatuses();
      if (statuses) setAgents(statuses);
    } catch {}
  }, []);

  const handleInit = useCallback(async (dir: string, roles: string[]) => {
    try {
      await WailsApp.InitProject(dir, roles);
      setProjectOpen(true);
      setLogs(prev => [...prev, `✅ 프로젝트 초기화 완료: ${dir}`]);
      await refreshStatuses();
    } catch (e: any) {
      setLogs(prev => [...prev, `❌ 초기화 실패: ${e}`]);
    }
  }, [refreshStatuses]);

  const handleOpen = useCallback(async (dir: string) => {
    try {
      await WailsApp.OpenProject(dir);
      setProjectOpen(true);
      setLogs(prev => [...prev, `✅ 프로젝트 열기 완료: ${dir}`]);
      await refreshStatuses();
    } catch (e: any) {
      setLogs(prev => [...prev, `❌ 프로젝트 열기 실패: ${e}`]);
    }
  }, [refreshStatuses]);

  const handleSubmit = useCallback(async (input: string) => {
    setRunning(true);
    setLogs(prev => [...prev, `\n📝 요청: ${input}`]);
    try {
      const result = await WailsApp.SubmitRequest(input);
      setReport(result);
      setShowReport(true);
      setLogs(prev => [...prev, '\n✅ 작업 완료 — 보고서를 확인하세요.']);
      await refreshStatuses();
    } catch (e: any) {
      setLogs(prev => [...prev, `❌ 오류: ${e}`]);
    }
    setRunning(false);
  }, [refreshStatuses]);

  const handleSelectDir = useCallback(async () => {
    try {
      return await WailsApp.SelectDirectory();
    } catch {
      return '';
    }
  }, []);

  if (!projectOpen) {
    return (
      <ProjectSetup
        onInit={handleInit}
        onOpen={handleOpen}
        onSelectDir={handleSelectDir}
      />
    );
  }

  return (
    <div style={{ display: 'flex', height: '100%' }}>
      <Sidebar
        agents={agents}
        onSelectAgent={setSelectedAgent}
        selectedAgent={selectedAgent}
      />

      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', position: 'relative' }}>
        {/* 헤더 */}
        <div style={{
          padding: '10px 16px',
          borderBottom: '1px solid var(--border)',
          background: 'var(--bg-secondary)',
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          fontSize: 13,
        }}>
          <span style={{ color: 'var(--text-muted)' }}>
            팀원 {agents.length}명 | {agents.filter(a => a.status === 'RUNNING').length}명 실행 중
          </span>
          <div style={{ display: 'flex', gap: 8 }}>
            {report && (
              <button
                onClick={() => setShowReport(!showReport)}
                style={{
                  padding: '4px 12px',
                  borderRadius: 4,
                  border: '1px solid var(--border)',
                  background: 'var(--bg-tertiary)',
                  color: 'var(--accent)',
                  cursor: 'pointer',
                  fontSize: 12,
                }}
              >
                📋 보고서
              </button>
            )}
          </div>
        </div>

        {/* 로그 */}
        <LogPanel logs={logs} />

        {/* 입력 */}
        <InputBar onSubmit={handleSubmit} disabled={running} />

        {/* 보고서 패널 */}
        <ReportPanel
          report={report}
          visible={showReport}
          onClose={() => setShowReport(false)}
        />
      </div>
    </div>
  );
}
