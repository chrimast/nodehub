import { fireEvent, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import NodesPage from '@/pages/nodes/NodesPage';
import { renderWithProviders } from './test-utils';

vi.mock('@/api/queries/useNodeMutations', () => ({
  useNodeMutations: () => ({
    applyAgentConfig: async () => {
      throw new Error('network failure');
    },
  }),
}));
vi.mock('@/api/queries/useNodesQuery', () => ({
  useNodesQuery: () => ({
    nodes: [],
    fetched: true,
    totals: { total: 0, online: 0, offline: 0, avgLatency: 0 },
  }),
}));
vi.mock('@/layouts/AppSidebar', () => ({ default: () => null }));
vi.mock('@/pages/nodes/NodeFormModal', () => ({ default: () => null }));
vi.mock('@/pages/nodes/NodeList', () => ({
  default: ({ onApplyAgentConfig }: { onApplyAgentConfig: (node: unknown) => Promise<string> }) => (
    <button
      onClick={async (e) => {
        const button = e.currentTarget;
        button.textContent = await onApplyAgentConfig({ id: 11, enable: true });
      }}
    >
      apply
    </button>
  ),
}));

describe('agent apply page status', () => {
  it('reports a request exception as unknown', async () => {
    renderWithProviders(<NodesPage />);
    fireEvent.click(screen.getByRole('button', { name: 'apply' }));
    expect(await screen.findByRole('button', { name: 'unknown' })).toBeTruthy();
  });
});
