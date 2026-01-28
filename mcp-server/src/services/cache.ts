/**
 * Cache service for storing fetched data with TTL
 */

import * as fs from "fs";
import * as path from "path";
import { CacheEntry } from "../types.js";

const CACHE_DIR = ".cache";
const DEFAULT_TTL = 3600000; // 1 hour in milliseconds

export class CacheService {
  private memoryCache: Map<string, CacheEntry<unknown>> = new Map();
  private cacheDir: string;

  constructor(cacheDir?: string) {
    this.cacheDir = cacheDir ?? path.join(process.cwd(), CACHE_DIR);
    this.ensureCacheDir();
  }

  private ensureCacheDir(): void {
    try {
      if (!fs.existsSync(this.cacheDir)) {
        fs.mkdirSync(this.cacheDir, { recursive: true });
      }
    } catch {
      // Cache dir creation failed, will use memory-only caching
      console.error("Warning: Could not create cache directory, using memory-only cache");
    }
  }

  /**
   * Get a value from cache
   */
  async get<T>(key: string): Promise<T | null> {
    // Check memory cache first
    const memEntry = this.memoryCache.get(key) as CacheEntry<T> | undefined;
    if (memEntry && !this.isExpired(memEntry)) {
      return memEntry.data;
    }

    // Check disk cache
    try {
      const filePath = this.getCacheFilePath(key);
      if (fs.existsSync(filePath)) {
        const content = fs.readFileSync(filePath, "utf-8");
        const entry: CacheEntry<T> = JSON.parse(content);
        if (!this.isExpired(entry)) {
          // Populate memory cache
          this.memoryCache.set(key, entry);
          return entry.data;
        } else {
          // Clean up expired cache
          fs.unlinkSync(filePath);
        }
      }
    } catch {
      // Cache read failed, continue without cache
    }

    return null;
  }

  /**
   * Set a value in cache
   */
  async set<T>(key: string, data: T, ttl: number = DEFAULT_TTL): Promise<void> {
    const entry: CacheEntry<T> = {
      data,
      timestamp: Date.now(),
      ttl,
    };

    // Always set in memory cache
    this.memoryCache.set(key, entry);

    // Try to persist to disk
    try {
      const filePath = this.getCacheFilePath(key);
      fs.writeFileSync(filePath, JSON.stringify(entry, null, 2));
    } catch {
      // Disk cache write failed, memory cache is still valid
    }
  }

  /**
   * Delete a value from cache
   */
  async delete(key: string): Promise<void> {
    this.memoryCache.delete(key);

    try {
      const filePath = this.getCacheFilePath(key);
      if (fs.existsSync(filePath)) {
        fs.unlinkSync(filePath);
      }
    } catch {
      // Ignore delete errors
    }
  }

  /**
   * Clear all cache
   */
  async clear(): Promise<void> {
    this.memoryCache.clear();

    try {
      if (fs.existsSync(this.cacheDir)) {
        const files = fs.readdirSync(this.cacheDir);
        for (const file of files) {
          fs.unlinkSync(path.join(this.cacheDir, file));
        }
      }
    } catch {
      // Ignore clear errors
    }
  }

  /**
   * Check if a cache entry is expired
   */
  private isExpired(entry: CacheEntry<unknown>): boolean {
    return Date.now() - entry.timestamp > entry.ttl;
  }

  /**
   * Get the file path for a cache key
   */
  private getCacheFilePath(key: string): string {
    // Sanitize key for use as filename
    const safeKey = key.replace(/[^a-zA-Z0-9_-]/g, "_");
    return path.join(this.cacheDir, `${safeKey}.json`);
  }
}
