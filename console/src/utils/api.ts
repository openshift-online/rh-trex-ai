type ListResponse<T> = {
  kind: string;
  page: number;
  size: number;
  total: number;
  items: T[];
};

type ListOptions = {
  page?: number;
  size?: number;
  search?: string;
  orderBy?: string;
};

type FilteredListOptions = ListOptions & {
  projectId?: string;
  entityDefinitionId?: string;
};

function getBaseURL(): string {
  if (typeof window !== 'undefined' && (window as Record<string, unknown>).__TREX_API_URL__) {
    return (window as Record<string, unknown>).__TREX_API_URL__ as string;
  }
  if (typeof process !== 'undefined' && process.env && process.env.REACT_APP_API_URL) {
    return `${process.env.REACT_APP_API_URL}/api/rh-trex-ai/v1`;
  }
  return '/api/proxy/plugin/trex-console/api/api/rh-trex-ai/v1';
}

async function apiFetch<T>(
  method: string,
  path: string,
  body?: unknown,
): Promise<T> {
  const opts: RequestInit = {
    method,
    headers: {
      Accept: 'application/json',
      ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}),
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  };

  let fetchFn: typeof fetch = fetch;
  try {
    const sdk = await import('@openshift-console/dynamic-plugin-sdk');
    if (sdk && sdk.consoleFetch) {
      fetchFn = sdk.consoleFetch as unknown as typeof fetch;
    }
  } catch {
    // standalone mode
  }

  const resp = await fetchFn(path, opts);

  if (!resp.ok) {
    const text = await resp.text().catch(() => resp.statusText);
    throw new Error(`API ${method} ${path} failed (${resp.status}): ${text}`);
  }

  if (resp.status === 204) {
    return undefined as T;
  }

  return resp.json();
}

function buildQuery(opts?: FilteredListOptions): string {
  if (!opts) return '';
  const params = new URLSearchParams();
  if (opts.page !== undefined) params.set('page', String(opts.page));
  if (opts.size !== undefined) params.set('size', String(opts.size));
  if (opts.search) params.set('search', opts.search);
  if (opts.orderBy) params.set('orderBy', opts.orderBy);
  if (opts.projectId) params.set('project_id', opts.projectId);
  if (opts.entityDefinitionId) params.set('entity_definition_id', opts.entityDefinitionId);
  const qs = params.toString();
  return qs ? `?${qs}` : '';
}

export function createAPIClient() {
  const base = getBaseURL();

  return {
    builds: {
      list: (opts?: FilteredListOptions) =>
        apiFetch<ListResponse<Record<string, unknown>>>('GET', `${base}/builds${buildQuery(opts)}`),
      get: (id: string) =>
        apiFetch<Record<string, unknown>>('GET', `${base}/builds/${id}`),
      create: (data: Record<string, unknown>) =>
        apiFetch<Record<string, unknown>>('POST', `${base}/builds`, data),
      delete: (id: string) =>
        apiFetch<void>('DELETE', `${base}/builds/${id}`),
    },
    dinosaurs: {
      list: (opts?: ListOptions) =>
        apiFetch<ListResponse<Record<string, unknown>>>('GET', `${base}/dinosaurs${buildQuery(opts)}`),
      get: (id: string) =>
        apiFetch<Record<string, unknown>>('GET', `${base}/dinosaurs/${id}`),
      create: (data: Record<string, unknown>) =>
        apiFetch<Record<string, unknown>>('POST', `${base}/dinosaurs`, data),
      update: (id: string, data: Record<string, unknown>) =>
        apiFetch<Record<string, unknown>>('PATCH', `${base}/dinosaurs/${id}`, data),
    },
    entityDefinitions: {
      list: (opts?: FilteredListOptions) =>
        apiFetch<ListResponse<Record<string, unknown>>>('GET', `${base}/entity_definitions${buildQuery(opts)}`),
      get: (id: string) =>
        apiFetch<Record<string, unknown>>('GET', `${base}/entity_definitions/${id}`),
      create: (data: Record<string, unknown>) =>
        apiFetch<Record<string, unknown>>('POST', `${base}/entity_definitions`, data),
      update: (id: string, data: Record<string, unknown>) =>
        apiFetch<Record<string, unknown>>('PATCH', `${base}/entity_definitions/${id}`, data),
      delete: (id: string) =>
        apiFetch<void>('DELETE', `${base}/entity_definitions/${id}`),
    },
    fieldDefinitions: {
      list: (opts?: FilteredListOptions) =>
        apiFetch<ListResponse<Record<string, unknown>>>('GET', `${base}/field_definitions${buildQuery(opts)}`),
      get: (id: string) =>
        apiFetch<Record<string, unknown>>('GET', `${base}/field_definitions/${id}`),
      create: (data: Record<string, unknown>) =>
        apiFetch<Record<string, unknown>>('POST', `${base}/field_definitions`, data),
      update: (id: string, data: Record<string, unknown>) =>
        apiFetch<Record<string, unknown>>('PATCH', `${base}/field_definitions/${id}`, data),
      delete: (id: string) =>
        apiFetch<void>('DELETE', `${base}/field_definitions/${id}`),
    },
    fossils: {
      list: (opts?: ListOptions) =>
        apiFetch<ListResponse<Record<string, unknown>>>('GET', `${base}/fossils${buildQuery(opts)}`),
      get: (id: string) =>
        apiFetch<Record<string, unknown>>('GET', `${base}/fossils/${id}`),
      create: (data: Record<string, unknown>) =>
        apiFetch<Record<string, unknown>>('POST', `${base}/fossils`, data),
      update: (id: string, data: Record<string, unknown>) =>
        apiFetch<Record<string, unknown>>('PATCH', `${base}/fossils/${id}`, data),
    },
    projects: {
      list: (opts?: ListOptions) =>
        apiFetch<ListResponse<Record<string, unknown>>>('GET', `${base}/projects${buildQuery(opts)}`),
      get: (id: string) =>
        apiFetch<Record<string, unknown>>('GET', `${base}/projects/${id}`),
      create: (data: Record<string, unknown>) =>
        apiFetch<Record<string, unknown>>('POST', `${base}/projects`, data),
      update: (id: string, data: Record<string, unknown>) =>
        apiFetch<Record<string, unknown>>('PATCH', `${base}/projects/${id}`, data),
      delete: (id: string) =>
        apiFetch<void>('DELETE', `${base}/projects/${id}`),
    },
    relationships: {
      list: (opts?: FilteredListOptions) =>
        apiFetch<ListResponse<Record<string, unknown>>>('GET', `${base}/relationships${buildQuery(opts)}`),
      get: (id: string) =>
        apiFetch<Record<string, unknown>>('GET', `${base}/relationships/${id}`),
      create: (data: Record<string, unknown>) =>
        apiFetch<Record<string, unknown>>('POST', `${base}/relationships`, data),
      update: (id: string, data: Record<string, unknown>) =>
        apiFetch<Record<string, unknown>>('PATCH', `${base}/relationships/${id}`, data),
      delete: (id: string) =>
        apiFetch<void>('DELETE', `${base}/relationships/${id}`),
    },
    scientists: {
      list: (opts?: ListOptions) =>
        apiFetch<ListResponse<Record<string, unknown>>>('GET', `${base}/scientists${buildQuery(opts)}`),
      get: (id: string) =>
        apiFetch<Record<string, unknown>>('GET', `${base}/scientists/${id}`),
      create: (data: Record<string, unknown>) =>
        apiFetch<Record<string, unknown>>('POST', `${base}/scientists`, data),
      update: (id: string, data: Record<string, unknown>) =>
        apiFetch<Record<string, unknown>>('PATCH', `${base}/scientists/${id}`, data),
    },
  };
}

export type APIClient = ReturnType<typeof createAPIClient>;
