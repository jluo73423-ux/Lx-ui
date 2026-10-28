import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import type { Status } from '@/models/status';
import VersionModal from '@/pages/index/VersionModal';
import { HttpUtil } from '@/utils';

describe('Xray version picker: REALITY client gates', () => {
  it('flags cores that silently reject REALITY clients', async () => {
    vi.spyOn(HttpUtil, 'get').mockResolvedValue({
      success: true,
      msg: '',
      obj: [
        { version: 'v26.10.21', tier: 'recommended', realityGate: 'mlkem' },
        { version: 'v26.7.28', tier: 'verified', realityGate: 'minver' },
        { version: 'v26.6.27', tier: 'verified', realityGate: 'none' },
      ],
    });
    render(
      <VersionModal
        open
        status={{ xray: { version: '26.10.21' } } as unknown as Status}
        onClose={vi.fn()}
        onBusy={vi.fn()}
      />,
    );
    expect(await screen.findByText('REALITY: ML-KEM only')).toBeTruthy();
    expect(screen.getByText('REALITY: min client 26.3.27')).toBeTruthy();
    // one tag each: the gate-free core carries none
    expect(screen.getAllByText(/^REALITY:/)).toHaveLength(2);
  });

  it('warns and asks for an explicit confirmation before switching to an ML-KEM core', async () => {
    vi.spyOn(HttpUtil, 'get').mockResolvedValue({
      success: true,
      msg: '',
      obj: [
        { version: 'v26.10.21', tier: 'verified', realityGate: 'mlkem' },
        { version: 'v26.6.27', tier: 'recommended', realityGate: 'none' },
      ],
    });
    const post = vi
      .spyOn(HttpUtil, 'post')
      .mockResolvedValue({ success: true, msg: '', obj: null });
    render(
      <VersionModal
        open
        status={{ xray: { version: '26.6.27' } } as unknown as Status}
        onClose={vi.fn()}
        onBusy={vi.fn()}
      />,
    );
    await screen.findByText('REALITY: ML-KEM only');
    fireEvent.click(screen.getAllByRole('radio')[0]);
    expect(
      await screen.findByText(/Shadowrocket, older Clash and sing-box will NOT work/),
    ).toBeTruthy();
    expect(screen.getByRole('button', { name: 'Switch anyway' })).toBeTruthy();
    expect(post).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole('button', { name: 'Switch anyway' }));
    await vi.waitFor(() =>
      expect(post).toHaveBeenCalledWith('/panel/api/server/installXray/v26.10.21'),
    );
  });
});
