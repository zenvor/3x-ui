export const keys = {
  server: {
    status: () => ['server', 'status'] as const,
  },
  nodes: {
    root: () => ['nodes'] as const,
    list: () => ['nodes', 'list'] as const,
  },
  settings: {
    root: () => ['settings'] as const,
    all: () => ['settings', 'all'] as const,
    defaults: () => ['settings', 'defaults'] as const,
  },
  inbounds: {
    root: () => ['inbounds'] as const,
    slim: () => ['inbounds', 'slim'] as const,
    options: () => ['inbounds', 'options'] as const,
  },
  clients: {
    root: () => ['clients'] as const,
    list: (params: unknown) => ['clients', 'list', params] as const,
    all: () => ['clients', 'all'] as const,
    onlines: () => ['clients', 'onlines'] as const,
    onlinesByGuid: () => ['clients', 'onlinesByGuid'] as const,
    activeInbounds: () => ['clients', 'activeInbounds'] as const,
    lastOnline: () => ['clients', 'lastOnline'] as const,
    groups: () => ['clients', 'groups'] as const,
  },
  xray: {
    root: () => ['xray'] as const,
    config: () => ['xray', 'config'] as const,
    outboundsTraffic: () => ['xray', 'outboundsTraffic'] as const,
  },
  subconverter: {
    root: () => ['subconverter'] as const,
    list: () => ['subconverter', 'list'] as const,
    detail: (id: number) => ['subconverter', 'detail', id] as const,
    settings: () => ['subconverter', 'settings'] as const,
    logs: (limit?: string | number) => {
      if (limit === undefined) return ['subconverter', 'logs'] as const;
      return ['subconverter', 'logs', String(limit)] as const;
    },
    inbounds: () => ['subconverter', 'inbounds'] as const,
    defaults: () => ['subconverter', 'defaults'] as const,
  },
} as const;
