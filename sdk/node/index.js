/**
 * Ledger Node.js Client
 * Connects to `ledger serve` JSON API
 */

class LedgerClient {
  /**
   * @param {Object} options
   * @param {string} [options.endpoint='http://localhost:7890'] - Ledger API endpoint
   */
  constructor(options = {}) {
    this.endpoint = (options.endpoint || 'http://localhost:7890').replace(/\/$/, '');
  }

  async _fetch(path, params = {}) {
    const url = new URL(path, this.endpoint);
    Object.entries(params).forEach(([k, v]) => {
      if (v !== undefined && v !== null && v !== '') {
        url.searchParams.set(k, String(v));
      }
    });

    const res = await fetch(url.toString());
    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      throw new Error(body.error || `HTTP ${res.status}`);
    }
    return res.json();
  }

  /**
   * Search indexed documents
   * @param {string} query - Search query
   * @param {Object} [options]
   * @param {number} [options.top=5] - Max results
   * @param {string} [options.type] - Filter by file type
   * @param {string} [options.tag] - Filter by tag
   * @param {string} [options.after] - Date filter (YYYY-MM-DD)
   * @param {string} [options.before] - Date filter (YYYY-MM-DD)
   * @param {string} [options.role] - Role-based boost
   * @returns {Promise<{query: string, results: Array, total: number}>}
   */
  async search(query, options = {}) {
    return this._fetch('/api/search', { q: query, ...options });
  }

  /**
   * Get index status
   * @returns {Promise<{files_indexed: number, total_chunks: number, embeddings: number, embed_model: string}>}
   */
  async status() {
    return this._fetch('/api/status');
  }

  /**
   * List chunks with optional filters
   * @param {Object} [options]
   * @param {string} [options.file] - Filter by file path
   * @param {string} [options.type] - Filter by file type
   * @param {number} [options.limit=50] - Max results
   * @param {number} [options.offset=0] - Offset
   * @returns {Promise<{chunks: Array, total: number}>}
   */
  async chunks(options = {}) {
    return this._fetch('/api/chunks', options);
  }

  /**
   * Run contradiction check
   * @param {Object} [options]
   * @param {number} [options.threshold=0.85] - Similarity threshold
   * @returns {Promise<{contradictions: Array, superseded: Array, total: number}>}
   */
  async check(options = {}) {
    return this._fetch('/api/check', options);
  }

  /**
   * Health check
   * @returns {Promise<{status: string, version: string}>}
   */
  async health() {
    return this._fetch('/api/health');
  }
}

module.exports = { LedgerClient };
