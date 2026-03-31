export interface SearchResult {
  id: number;
  file: string;
  heading: string;
  content: string;
  file_type: string;
  date: string;
  tags: string;
  line_start: number;
  line_end: number;
  score: number;
}

export interface SearchResponse {
  query: string;
  results: SearchResult[];
  total: number;
}

export interface StatusResponse {
  files_indexed: number;
  total_chunks: number;
  embeddings: number;
  embed_model: string;
}

export interface Chunk {
  id: number;
  file: string;
  heading: string;
  content: string;
  file_type: string;
  date: string;
  tags: string;
  line_start: number;
  line_end: number;
}

export interface ChunksResponse {
  chunks: Chunk[];
  total: number;
}

export interface Contradiction {
  file_a: string;
  heading_a: string;
  file_b: string;
  heading_b: string;
  similarity: number;
  reason: string;
}

export interface Superseded {
  older_file: string;
  older_heading: string;
  older_date: string;
  newer_file: string;
  newer_heading: string;
  newer_date: string;
  similarity: number;
}

export interface CheckResponse {
  contradictions: Contradiction[];
  superseded: Superseded[];
  total: number;
}

export interface HealthResponse {
  status: string;
  version: string;
}

export interface SearchOptions {
  top?: number;
  type?: string;
  tag?: string;
  after?: string;
  before?: string;
  role?: string;
}

export interface ChunkOptions {
  file?: string;
  type?: string;
  limit?: number;
  offset?: number;
}

export interface ClientOptions {
  endpoint?: string;
}

export class LedgerClient {
  constructor(options?: ClientOptions);
  search(query: string, options?: SearchOptions): Promise<SearchResponse>;
  status(): Promise<StatusResponse>;
  chunks(options?: ChunkOptions): Promise<ChunksResponse>;
  check(options?: { threshold?: number }): Promise<CheckResponse>;
  health(): Promise<HealthResponse>;
}
