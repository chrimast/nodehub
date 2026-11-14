import type { RealityScanResult } from '@/generated/types';

const REALITY_NEIGHBOR_LIMIT = 20;

type RealityScanResponse = {
  results?: unknown;
};

export function buildRealityNeighborScanPayload(nodeId: number | null) {
  return {
    targets: '@local24',
    ...(typeof nodeId === 'number' ? { nodeId } : {}),
    limit: REALITY_NEIGHBOR_LIMIT,
  };
}

export function extractRealityScanResults(response: unknown): RealityScanResult[] {
  if (Array.isArray(response)) return response as RealityScanResult[];
  if (
    response &&
    typeof response === 'object' &&
    Array.isArray((response as RealityScanResponse).results)
  ) {
    return (response as RealityScanResponse).results as RealityScanResult[];
  }
  return [];
}

export function selectRealityServerNames(result: RealityScanResult): string[] {
  const names = result.serverNames.map((name) => name.trim()).filter((name) => name !== '');
  if (names.length > 0) return Array.from(new Set(names));
  const host = result.host.trim();
  return host ? [host] : [];
}
