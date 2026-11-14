import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import NodeList from '@/pages/nodes/NodeList';
import NodeFormModal from '@/pages/nodes/NodeFormModal';
import { NodeFormSchema, NodeRecordSchema } from '@/schemas/node';
import { chooseSelectOption, renderWithProviders } from './test-utils';

const base = {
  name: 'agent',
  nodeType: 'agent',
  remark: '',
  scheme: 'https',
  address: 'example.com',
  port: 2053,
  basePath: '/',
  apiToken: 'token',
  hasStoredToken: false,
  enable: true,
  allowPrivateAddress: false,
  tlsVerifyMode: 'verify',
  pinnedCertSha256: '',
  inboundSyncMode: 'all',
  inboundTags: [],
  outboundTag: '',
};

describe('experimental agent nodes', () => {
  it('accepts HTTP and HTTPS while enforcing bearer authentication', () => {
    expect(NodeRecordSchema.parse({ id: 1, nodeType: 'agent' }).nodeType).toBe('agent');
    expect(NodeFormSchema.safeParse(base).success).toBe(true);
    expect(NodeFormSchema.safeParse({ ...base, scheme: 'http' }).success).toBe(true);
    expect(NodeFormSchema.safeParse({ ...base, tlsVerifyMode: 'mtls', apiToken: '' }).success).toBe(
      false,
    );
    expect(NodeFormSchema.safeParse({ ...base, apiToken: '', hasStoredToken: true }).success).toBe(
      true,
    );
  });

  it('warns about unencrypted credentials and configuration when HTTP is selected', async () => {
    renderWithProviders(
      <NodeFormModal
        open
        mode="add"
        node={null}
        testConnection={vi.fn()}
        fetchFingerprint={vi.fn()}
        fetchInbounds={vi.fn()}
        save={vi.fn()}
        onOpenChange={vi.fn()}
      />,
    );
    chooseSelectOption(screen.getByLabelText('Node type').id, 'Agent (experimental)');
    chooseSelectOption(screen.getByLabelText('Scheme').id, 'http');
    expect(await screen.findByText(/credentials and configuration.*unencrypted/i)).toBeTruthy();
  });

  it('shows agent readiness without offering panel updates', () => {
    const noop = () => {};
    renderWithProviders(
      <NodeList
        nodes={[
          {
            id: 1,
            name: 'agent-one',
            nodeType: 'agent',
            enable: true,
            status: 'online',
            xrayState: 'stop',
          },
          { id: 2, name: 'panel-one', enable: true, status: 'online' },
        ]}
        selectedIds={[]}
        onSelectionChange={noop}
        onAdd={noop}
        onMtls={noop}
        onEdit={noop}
        onDelete={noop}
        onProbe={noop}
        onToggleEnable={noop}
        onUpdateNode={noop}
        onUpdateSelected={noop}
      />,
    );
    const agent = screen.getByText('agent-one').closest('tr')!;
    const panel = screen.getByText('panel-one').closest('tr')!;
    expect(within(agent).queryByRole('button', { name: /update panel/i })).toBeNull();
    expect(within(panel).getByRole('button', { name: /update panel/i })).toBeTruthy();
    expect(within(agent).getByText(/agent connected/i)).toBeTruthy();
    expect((within(agent).getByRole('checkbox') as HTMLInputElement).disabled).toBe(true);
  });

  it('confirms agent config apply and reports only acknowledged results', async () => {
    const onApplyAgentConfig = vi.fn().mockResolvedValue('applied');
    const noop = () => {};
    renderWithProviders(
      <NodeList
        nodes={[{ id: 11, name: 'apply-agent', nodeType: 'agent', enable: true, status: 'online' }]}
        selectedIds={[]}
        onSelectionChange={noop}
        onAdd={noop}
        onMtls={noop}
        onEdit={noop}
        onDelete={noop}
        onProbe={noop}
        onToggleEnable={noop}
        onUpdateNode={noop}
        onUpdateSelected={noop}
        onApplyAgentConfig={onApplyAgentConfig}
      />,
    );

    const row = screen.getByText('apply-agent').closest('tr')!;
    fireEvent.click(within(row).getByRole('button', { name: /apply agent config/i }));
    expect(onApplyAgentConfig).not.toHaveBeenCalled();
    fireEvent.click(
      await within(await screen.findByRole('dialog')).getByRole('button', {
        name: 'Apply agent config',
      }),
    );
    await waitFor(() =>
      expect(onApplyAgentConfig).toHaveBeenCalledWith(expect.objectContaining({ id: 11 })),
    );
    expect(await within(row).findByText(/applied/i)).toBeTruthy();
  });

  it('does not apply a disabled agent and reports its disabled state', () => {
    const onApplyAgentConfig = vi.fn();
    const noop = () => {};
    renderWithProviders(
      <NodeList
        nodes={[
          { id: 12, name: 'disabled-agent', nodeType: 'agent', enable: false, status: 'unknown' },
        ]}
        selectedIds={[]}
        onSelectionChange={noop}
        onAdd={noop}
        onMtls={noop}
        onEdit={noop}
        onDelete={noop}
        onProbe={noop}
        onToggleEnable={noop}
        onUpdateNode={noop}
        onUpdateSelected={noop}
        onApplyAgentConfig={onApplyAgentConfig}
      />,
    );

    const row = screen.getByText('disabled-agent').closest('tr')!;
    expect(within(row).queryByRole('button', { name: /apply agent config/i })).toBeNull();
    expect(within(row).getByText('Disabled in panel; remote state unknown')).toBeTruthy();
    expect(onApplyAgentConfig).not.toHaveBeenCalled();
  });

  it('keeps agent apply in the mobile overflow menu', async () => {
    const onApplyAgentConfig = vi.fn().mockResolvedValue('unknown');
    const noop = () => {};
    renderWithProviders(
      <NodeList
        isMobile
        nodes={[
          { id: 13, name: 'mobile-agent', nodeType: 'agent', enable: true, status: 'online' },
        ]}
        selectedIds={[]}
        onSelectionChange={noop}
        onAdd={noop}
        onMtls={noop}
        onEdit={noop}
        onDelete={noop}
        onProbe={noop}
        onToggleEnable={noop}
        onUpdateNode={noop}
        onUpdateSelected={noop}
        onApplyAgentConfig={onApplyAgentConfig}
      />,
    );

    fireEvent.click(screen.getByRole('button', { name: 'more' }));
    fireEvent.click(await screen.findByText('Apply agent config'));
    fireEvent.click(
      await within(await screen.findByRole('dialog')).getByRole('button', {
        name: 'Apply agent config',
      }),
    );
    await waitFor(() =>
      expect(onApplyAgentConfig).toHaveBeenCalledWith(expect.objectContaining({ id: 13 })),
    );
    expect(await screen.findByText('Unknown')).toBeTruthy();
  });

  it('submits HTTP agent form without panel sync settings', async () => {
    const testConnection = vi.fn().mockResolvedValue({ success: true, obj: { status: 'online' } });
    const save = vi.fn().mockResolvedValue({ success: true });
    renderWithProviders(
      <NodeFormModal
        open
        mode="add"
        node={null}
        testConnection={testConnection}
        fetchFingerprint={vi.fn()}
        fetchInbounds={vi.fn()}
        save={save}
        onOpenChange={vi.fn()}
      />,
    );
    chooseSelectOption(screen.getByLabelText('Node type').id, 'Agent (experimental)');
    chooseSelectOption(screen.getByLabelText('Scheme').id, 'http');
    expect(screen.queryByText('Inbound Sync Mode')).toBeNull();
    fireEvent.change(screen.getByLabelText('Name'), { target: { value: 'agent-one' } });
    fireEvent.change(screen.getByLabelText('Address'), { target: { value: 'example.com' } });
    fireEvent.change(screen.getByLabelText('API Token'), { target: { value: 'secret' } });
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(save).toHaveBeenCalled());
    expect(save.mock.calls[0][0]).toMatchObject({
      nodeType: 'agent',
      scheme: 'http',
      apiToken: 'secret',
    });
    expect(save.mock.calls[0][0]).not.toHaveProperty('inboundSyncMode');
    expect(testConnection.mock.calls[0][0].nodeType).toBe('agent');
  });

  it('keeps a stored token when editing an agent', async () => {
    const testConnection = vi.fn().mockResolvedValue({ success: true, obj: { status: 'online' } });
    const save = vi.fn().mockResolvedValue({ success: true });
    renderWithProviders(
      <NodeFormModal
        open
        mode="edit"
        node={{
          id: 3,
          name: 'existing',
          nodeType: 'agent',
          scheme: 'https',
          address: 'example.com',
          port: 9443,
          basePath: '/',
          hasApiToken: true,
          enable: true,
        }}
        testConnection={testConnection}
        fetchFingerprint={vi.fn()}
        fetchInbounds={vi.fn()}
        save={save}
        onOpenChange={vi.fn()}
      />,
    );
    expect((screen.getByLabelText('Node type') as HTMLInputElement).disabled).toBe(true);
    expect(screen.queryByText('Inbound Sync Mode')).toBeNull();
    fireEvent.click(screen.getByRole('button', { name: 'Save' }));
    await waitFor(() => expect(save).toHaveBeenCalled());
    expect(save.mock.calls[0][0]).toMatchObject({ id: 3, nodeType: 'agent', scheme: 'https' });
    expect(save.mock.calls[0][0]).not.toHaveProperty('apiToken');
  });
});
