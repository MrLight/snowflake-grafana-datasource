import { DataQuery, DataSourceJsonData } from '@grafana/data';

export interface SnowflakeQuery extends DataQuery {
  queryText?: string;
  queryType?: string;
  timeColumns?: string[];
  fillMode?: string;
}

export const defaultQuery: Partial<SnowflakeQuery> = {
  queryText:
    'SELECT $__timeGroup(timestamp,$__interval ,previous) as time, count(*) as nb \n' +
    'FROM TABLE WHERE $__timeFilter(timestamp) \n' +
    'GROUP BY time ORDER BY time',
  queryType: 'table',
  timeColumns: ['time'],
  fillMode: 'null'
};

/**
 * These are options configured for each Snowflake instance
 */
export interface SnowflakeOptions extends DataSourceJsonData {
  account?: string;
  username?: string;
  role?: string;
  warehouse?: string;
  database?: string;
  schema?: string;
  extraConfig?: string;
  basicAuth: boolean;
  maxOpenConnections?: string;
  maxQueuedQueries?: string;
  connectionLifetime?: string;
  useCaching: boolean;
  useCacheByDefault: boolean;
  cacheSize?: string;
  cacheRetention?: string;
  maxChunkDownloadWorkers?: string;
  customJSONDecoderEnabled: boolean;
}

/**
 * Value that is used in the backend, but never sent over HTTP to the frontend
 */
export interface SnowflakeSecureOptions {
  password?: string;
  privateKey?: string;
}
