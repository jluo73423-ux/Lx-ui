import { render, screen } from '@testing-library/react';
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
        { version: 'v26.10.10', tier: 'recommended', realityGate: 'mlkem' },
        { version: 'v26.7.28', tier: 'verified', realityGate: 'minver' },
        { version: 'v26.6.27', tier: 'verified', realityGate: 'none' },
      ],
    });
    render(
      <VersionModal
        open
        status={{ xray: { version: '26.10.10' } } as unknown as Status}
        onClose={vi.fn()}
        onBusy={vi.fn()}
      />,
    );
    expect(await screen.findByText('REALITY: ML-KEM only')).toBeTruthy();
    expect(screen.getByText('REALITY: min client 26.3.27')).toBeTruthy();
    // one tag each: the gate-free core carries none
    expect(screen.getAllByText(/^REALITY:/)).toHaveLength(2);
  });
});
