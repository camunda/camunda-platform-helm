/**
 * DNS Fallback Preload Script
 *
 * Loaded via NODE_OPTIONS="--require ./dns-fallback.cjs" when the system DNS
 * resolver has a stale NXDOMAIN cached for a hostname that public DNS can
 * already resolve.
 *
 * Monkey-patches both:
 *   - dns.lookup          (callback-based, used by Node http/https)
 *   - dns.promises.lookup (Promise-based, used by Playwright's Happy Eyeballs agent)
 *
 * to fall back to Cloudflare (1.1.1.1), Google (8.8.8.8), and Quad9 (9.9.9.9)
 * DNS when the system resolver returns ENOTFOUND.
 *
 * This avoids needing sudo or /etc/hosts edits.
 */
"use strict";

const dns = require("dns");
const { Resolver } = dns;

const publicResolver = new Resolver();
publicResolver.setServers(["1.1.1.1", "8.8.8.8", "9.9.9.9"]);

/**
 * Discover the authoritative nameservers for a hostname by walking up the
 * domain hierarchy and looking for NS records.  Returns a Resolver instance
 * configured to use them, or null if discovery fails.
 *
 * This bypasses negative-cache TTLs (SOA minimum = 300s for this zone)
 * because authoritative servers always answer with the current zone state.
 */
function _discoverAuthoritativeNS(hostname) {
  return new Promise((resolve) => {
    const labels = hostname.split(".");
    let idx = 1; // skip the first label (the subdomain itself)

    function tryNext() {
      if (idx >= labels.length - 1) {
        return resolve(null); // exhausted hierarchy
      }
      const domain = labels.slice(idx).join(".");
      idx++;
      publicResolver.resolveNs(domain, (err, nsRecords) => {
        if (!err && nsRecords && nsRecords.length > 0) {
          const authResolver = new Resolver();
          authResolver.setServers(nsRecords);
          return resolve(authResolver);
        }
        tryNext();
      });
    }
    tryNext();
  });
}

/**
 * Resolve hostname via public DNS, then authoritative NS as a last resort.
 * Returns an array of IPv4 addresses or throws the original error.
 */
async function _resolveWithFallback(hostname, origErr) {
  // Try public recursive resolvers first
  const publicAddrs = await new Promise((resolve) => {
    publicResolver.resolve4(hostname, (err, addrs) => {
      if (!err && addrs && addrs.length > 0) return resolve(addrs);
      resolve(null);
    });
  });
  if (publicAddrs) return publicAddrs;

  // Public DNS also has stale NXDOMAIN — try authoritative NS directly
  const authResolver = await _discoverAuthoritativeNS(hostname);
  if (authResolver) {
    const authAddrs = await new Promise((resolve) => {
      authResolver.resolve4(hostname, (err, addrs) => {
        if (!err && addrs && addrs.length > 0) return resolve(addrs);
        resolve(null);
      });
    });
    if (authAddrs) return authAddrs;
  }

  throw origErr;
}

// ---------------------------------------------------------------------------
// 1. Patch callback-based dns.lookup (used by Node's http/https Agent)
// ---------------------------------------------------------------------------
const origLookup = dns.lookup;

dns.lookup = function (hostname, options, callback) {
  // Normalise arguments — options is optional
  if (typeof options === "function") {
    callback = options;
    options = {};
  } else if (typeof options === "number") {
    options = { family: options };
  }

  origLookup.call(dns, hostname, options, (err, address, family) => {
    if (err && err.code === "ENOTFOUND") {
      // System resolver failed — try public DNS via c-ares
      resolver.resolve4(hostname, (resolveErr, addresses) => {
        if (resolveErr || !addresses || addresses.length === 0) {
          // Public DNS also failed — return the original error
          return callback(err);
        }
        // If the caller asked for { all: true }, return an array
        if (options && options.all) {
          return callback(
            null,
            addresses.map((a) => ({ address: a, family: 4 }))
          );
        }
        callback(null, addresses[0], 4);
      });
    } else {
      callback(err, address, family);
    }
  });
};

// ---------------------------------------------------------------------------
// 2. Patch Promise-based dns.promises.lookup (used by Playwright)
//
//    Playwright's Happy Eyeballs agent calls:
//      dns.promises.lookup(hostname, { all: true, family: 0, verbatim: true })
//    which returns Promise<Array<{ address: string, family: number }>>
//
//    dns.promises is a separate object — patching dns.lookup does NOT
//    automatically patch dns.promises.lookup.
// ---------------------------------------------------------------------------
const dnsPromises = dns.promises;
const origPromisesLookup = dnsPromises.lookup;

dnsPromises.lookup = async function (hostname, options) {
  try {
    return await origPromisesLookup.call(dnsPromises, hostname, options);
  } catch (err) {
    if (err && err.code === "ENOTFOUND") {
      // System resolver failed — try public DNS via c-ares Resolver
      // resolver.resolve4 is callback-based; wrap in a promise
      const addresses = await new Promise((resolve, reject) => {
        resolver.resolve4(hostname, (resolveErr, addrs) => {
          if (resolveErr || !addrs || addrs.length === 0) {
            return reject(err); // reject with the *original* ENOTFOUND error
          }
          resolve(addrs);
        });
      });

      const opts =
        typeof options === "number" ? { family: options } : options || {};

      // When { all: true } — return array of { address, family } objects
      if (opts.all) {
        return addresses.map((a) => ({ address: a, family: 4 }));
      }
      // Single result — return { address, family }
      return { address: addresses[0], family: 4 };
    }
    throw err;
  }
};
