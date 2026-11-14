import { describe, expect, it } from 'vitest';

import {
  buildRealityNeighborScanPayload,
  extractRealityScanResults,
  selectRealityServerNames,
} from '@/pages/inbounds/form/security/reality-scan';
import type { RealityScanResult } from '@/generated/types';

const result: RealityScanResult = {
  alpn: 'h2',
  certChainBytes: 4000,
  certChainValid: true,
  certIssuer: 'issuer',
  certSubject: 'subject',
  certValid: true,
  curveID: 'X25519',
  feasible: true,
  h2: true,
  host: 'front.example.com',
  ip: '198.51.100.10',
  latencyMs: 20,
  notAfter: '',
  port: 443,
  privateTarget: false,
  reason: '',
  serverNames: ['front.example.com', '*.example.com'],
  target: '198.51.100.10:443',
  tls13: true,
  tlsVersion: '1.3',
  x25519: true,
};

describe('REALITY neighbor scan contract', () => {
  it('requests the local /24 and carries a selected inbound node id', () => {
    expect(buildRealityNeighborScanPayload(12)).toEqual({
      targets: '@local24',
      nodeId: 12,
      limit: 20,
    });
  });

  it('keeps the local panel scan local when no node is selected', () => {
    expect(buildRealityNeighborScanPayload(null)).toEqual({
      targets: '@local24',
      limit: 20,
    });
  });

  it('unwraps the neighbor response while retaining compatibility with direct arrays', () => {
    expect(extractRealityScanResults({ results: [result] })).toEqual([result]);
    expect(extractRealityScanResults([result])).toEqual([result]);
    expect(extractRealityScanResults({ scanned: 253 })).toEqual([]);
  });

  it('filters empty SAN entries and falls back to the probed host for SNI', () => {
    expect(selectRealityServerNames({ ...result, serverNames: ['', '  '] })).toEqual([
      'front.example.com',
    ]);
    expect(selectRealityServerNames({ ...result, host: '', serverNames: ['', '  '] })).toEqual([]);
  });
});
