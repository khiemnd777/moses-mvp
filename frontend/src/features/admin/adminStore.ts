import { create } from 'zustand';

export type AdminState = {
  selectedDocTypeId?: string;
  lastOpenedDocumentId?: string;
  setSelectedDocTypeId: (id?: string) => void;
  setLastOpenedDocumentId: (id?: string) => void;
};

export const useAdminStore = create<AdminState>((set) => ({
  selectedDocTypeId: undefined,
  lastOpenedDocumentId: undefined,
  setSelectedDocTypeId: (id) => set({ selectedDocTypeId: id }),
  setLastOpenedDocumentId: (id) => set({ lastOpenedDocumentId: id })
}));
