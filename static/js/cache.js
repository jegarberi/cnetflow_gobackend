/**
 * LocalStorage Cache Manager for NetFlow Application
 * Provides efficient caching with TTL (Time To Live) support
 */

class CacheManager {
    constructor(prefix = 'netflow_', defaultTTL = 5 * 60 * 1000) { // Default 5 minutes
        this.prefix = prefix;
        this.defaultTTL = defaultTTL;
        this.cleanupOldEntries();
    }

    /**
     * Generate a cache key from request parameters
     */
    generateKey(endpoint, params = {}) {
        const sortedParams = Object.keys(params)
            .sort()
            .map(key => `${key}=${params[key]}`)
            .join('&');
        return `${this.prefix}${endpoint}?${sortedParams}`;
    }

    /**
     * Store data in cache with TTL
     */
    set(key, data, ttl = this.defaultTTL) {
        try {
            const cacheEntry = {
                data: data,
                timestamp: Date.now(),
                ttl: ttl,
                expires: Date.now() + ttl
            };

            localStorage.setItem(key, JSON.stringify(cacheEntry));
            this.updateCacheMetadata(key);
            return true;
        } catch (error) {
            console.warn('Cache set failed:', error);
            // If storage is full, clear old entries and retry
            if (error.name === 'QuotaExceededError') {
                this.clearOldest(5);
                try {
                    localStorage.setItem(key, JSON.stringify({
                        data: data,
                        timestamp: Date.now(),
                        ttl: ttl,
                        expires: Date.now() + ttl
                    }));
                    return true;
                } catch (retryError) {
                    console.error('Cache set retry failed:', retryError);
                }
            }
            return false;
        }
    }

    /**
     * Retrieve data from cache if not expired
     */
    get(key) {
        try {
            const item = localStorage.getItem(key);
            if (!item) return null;

            const cacheEntry = JSON.parse(item);

            // Check if expired
            if (Date.now() > cacheEntry.expires) {
                this.remove(key);
                return null;
            }

            // Update access time
            this.updateAccessTime(key);
            return cacheEntry.data;
        } catch (error) {
            console.warn('Cache get failed:', error);
            this.remove(key);
            return null;
        }
    }

    /**
     * Check if cache entry exists and is valid
     */
    has(key) {
        return this.get(key) !== null;
    }

    /**
     * Remove specific cache entry
     */
    remove(key) {
        try {
            localStorage.removeItem(key);
            this.removeFromMetadata(key);
        } catch (error) {
            console.warn('Cache remove failed:', error);
        }
    }

    /**
     * Clear all cache entries for this app
     */
    clear() {
        try {
            const keys = Object.keys(localStorage);
            keys.forEach(key => {
                if (key.startsWith(this.prefix)) {
                    localStorage.removeItem(key);
                }
            });
            localStorage.removeItem(`${this.prefix}metadata`);
            console.log('Cache cleared');
        } catch (error) {
            console.warn('Cache clear failed:', error);
        }
    }

    /**
     * Clean up expired entries
     */
    cleanupOldEntries() {
        try {
            const keys = Object.keys(localStorage);
            let cleaned = 0;

            keys.forEach(key => {
                if (key.startsWith(this.prefix) && key !== `${this.prefix}metadata`) {
                    try {
                        const item = localStorage.getItem(key);
                        if (item) {
                            const cacheEntry = JSON.parse(item);
                            if (Date.now() > cacheEntry.expires) {
                                localStorage.removeItem(key);
                                cleaned++;
                            }
                        }
                    } catch (e) {
                        localStorage.removeItem(key);
                        cleaned++;
                    }
                }
            });

            if (cleaned > 0) {
                console.log(`Cleaned ${cleaned} expired cache entries`);
            }
        } catch (error) {
            console.warn('Cache cleanup failed:', error);
        }
    }

    /**
     * Clear oldest cache entries
     */
    clearOldest(count = 5) {
        try {
            const metadata = this.getMetadata();
            const sorted = metadata.sort((a, b) => a.lastAccess - b.lastAccess);

            for (let i = 0; i < Math.min(count, sorted.length); i++) {
                this.remove(sorted[i].key);
            }
        } catch (error) {
            console.warn('Clear oldest failed:', error);
        }
    }

    /**
     * Get cache statistics
     */
    getStats() {
        let totalSize = 0;
        let count = 0;
        const keys = Object.keys(localStorage);

        keys.forEach(key => {
            if (key.startsWith(this.prefix)) {
                const item = localStorage.getItem(key);
                if (item) {
                    totalSize += item.length;
                    count++;
                }
            }
        });

        return {
            entries: count,
            sizeBytes: totalSize,
            sizeKB: (totalSize / 1024).toFixed(2),
            sizeMB: (totalSize / (1024 * 1024)).toFixed(2)
        };
    }

    /**
     * Update cache metadata for tracking
     */
    updateCacheMetadata(key) {
        try {
            const metadata = this.getMetadata();
            const existing = metadata.find(m => m.key === key);

            if (existing) {
                existing.lastAccess = Date.now();
                existing.accessCount++;
            } else {
                metadata.push({
                    key: key,
                    created: Date.now(),
                    lastAccess: Date.now(),
                    accessCount: 1
                });
            }

            localStorage.setItem(`${this.prefix}metadata`, JSON.stringify(metadata));
        } catch (error) {
            console.warn('Metadata update failed:', error);
        }
    }

    /**
     * Update access time for cache entry
     */
    updateAccessTime(key) {
        try {
            const metadata = this.getMetadata();
            const entry = metadata.find(m => m.key === key);

            if (entry) {
                entry.lastAccess = Date.now();
                entry.accessCount++;
                localStorage.setItem(`${this.prefix}metadata`, JSON.stringify(metadata));
            }
        } catch (error) {
            console.warn('Access time update failed:', error);
        }
    }

    /**
     * Get cache metadata
     */
    getMetadata() {
        try {
            const metadata = localStorage.getItem(`${this.prefix}metadata`);
            return metadata ? JSON.parse(metadata) : [];
        } catch (error) {
            console.warn('Get metadata failed:', error);
            return [];
        }
    }

    /**
     * Remove key from metadata
     */
    removeFromMetadata(key) {
        try {
            const metadata = this.getMetadata();
            const filtered = metadata.filter(m => m.key !== key);
            localStorage.setItem(`${this.prefix}metadata`, JSON.stringify(filtered));
        } catch (error) {
            console.warn('Remove from metadata failed:', error);
        }
    }

    /**
     * Invalidate cache entries matching a pattern
     */
    invalidatePattern(pattern) {
        try {
            const keys = Object.keys(localStorage);
            let invalidated = 0;

            keys.forEach(key => {
                if (key.startsWith(this.prefix) && key.includes(pattern)) {
                    this.remove(key);
                    invalidated++;
                }
            });

            console.log(`Invalidated ${invalidated} cache entries matching: ${pattern}`);
        } catch (error) {
            console.warn('Invalidate pattern failed:', error);
        }
    }

    /**
     * Fetch with cache
     */
    async fetchWithCache(url, options = {}, ttl = this.defaultTTL) {
        const cacheKey = this.generateKey(url, options.params || {});

        // Try to get from cache first
        const cached = this.get(cacheKey);
        if (cached) {
            console.log('Cache hit:', cacheKey);
            return { data: cached, fromCache: true };
        }

        // Fetch from server
        console.log('Cache miss, fetching:', url);
        try {
            const response = await fetch(url);
            if (!response.ok) {
                throw new Error(`HTTP ${response.status}: ${response.statusText}`);
            }

            const data = await response.json();

            // Store in cache
            this.set(cacheKey, data, ttl);

            return { data: data, fromCache: false };
        } catch (error) {
            console.error('Fetch error:', error);
            throw error;
        }
    }
}

// Create global cache instance
const cache = new CacheManager('netflow_', 5 * 60 * 1000); // 5 minutes default TTL

// Cache configuration presets
const CacheTTL = {
    SHORT: 1 * 60 * 1000,      // 1 minute
    MEDIUM: 5 * 60 * 1000,     // 5 minutes
    LONG: 15 * 60 * 1000,      // 15 minutes
    HOUR: 60 * 60 * 1000,      // 1 hour
    DAY: 24 * 60 * 60 * 1000   // 24 hours
};

// Export for use in HTML pages
if (typeof window !== 'undefined') {
    window.CacheManager = CacheManager;
    window.cache = cache;
    window.CacheTTL = CacheTTL;
}

// Auto-cleanup every 5 minutes
setInterval(() => {
    cache.cleanupOldEntries();
}, 5 * 60 * 1000);

// Show cache stats in console (for debugging)
console.log('Cache Manager initialized. Stats:', cache.getStats());
