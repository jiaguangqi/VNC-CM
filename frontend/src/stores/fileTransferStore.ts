import { create } from 'zustand';

export type TransferStatus = 'pending' | 'running' | 'done' | 'error';

export interface TransferTask {
  id: string;
  type: 'upload' | 'download';
  filename: string;
  progress: number;
  status: TransferStatus;
  error?: string;
}

interface FileTransferState {
  visible: boolean;
  minimized: boolean;
  desktopId: string | null;
  desktopName: string;
  tasks: TransferTask[];

  open: (desktopId: string, desktopName: string) => void;
  close: () => void;
  minimize: () => void;
  restore: () => void;

  addTask: (task: Omit<TransferTask, 'id'>) => string;
  updateTask: (id: string, patch: Partial<TransferTask>) => void;
  removeTask: (id: string) => void;
  clearDone: () => void;
}

let idSeq = 0;
const genId = () => `t_${Date.now()}_${++idSeq}`;

export const useFileTransferStore = create<FileTransferState>()((set, get) => ({
  visible: false,
  minimized: false,
  desktopId: null,
  desktopName: '',
  tasks: [],

  open: (desktopId, desktopName) =>
    set({ visible: true, minimized: false, desktopId, desktopName }),

  close: () => set({ visible: false }),

  minimize: () => set({ minimized: true, visible: false }),

  restore: () => set({ minimized: false, visible: true }),

  addTask: (task) => {
    const id = genId();
    set((s) => ({ tasks: [...s.tasks, { ...task, id }] }));
    return id;
  },

  updateTask: (id, patch) =>
    set((s) => ({
      tasks: s.tasks.map((t) => (t.id === id ? { ...t, ...patch } : t)),
    })),

  removeTask: (id) =>
    set((s) => ({ tasks: s.tasks.filter((t) => t.id !== id) })),

  clearDone: () =>
    set((s) => ({
      tasks: s.tasks.filter((t) => t.status !== 'done'),
    })),
}));
