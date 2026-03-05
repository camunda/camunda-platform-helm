/**
 * DNS Fallback Preload Script
 *
 * Loaded via NODE_OPTIONS="--require ./dns-fallback.cjs" when the system DNS
 * resolver has a stale NXDOMAIN cached for a hostname that public DNS can
 * already resolve.
 *
 * Two resolution strategies, tried in order:
 *
 * 1. **Static override** (fast & reliable) — when the caller sets
 *      DNS_FALLBACK_HOSTNAMES="host1,host2"  DNS_FALLBACK_IP="1.2.3.4"
 *    or a full mapping
 *      DNS_FALLBACK_MAP="host1=1.2.3.4,host2=5.6.7.8"
 *    any ENOTFOUND for those hostnames is resolved instantly from the map.
 *
 * 2. **Public recursive resolvers** — Cloudflare 1.1.1.1, Google 8.8.8.8,
 *    Quad9 9.9.9.9.
 *
 * 3. **Authoritative NS** — walks up the domain hierarchy and queries the
 *    zone's own nameservers, bypassing negative-cache TTLs entirely.
 *
 * Monkey-patches both:
 *   - dns.lookup          (callback-based, used by Node http/https)
 *   - dns.promises.lookup (Promise-based, used by Playwright's Happy Eyeballs agent)
 *
 * This avoids needing sudo or /etc/hosts edits.
 */
"use strict";

const dns = require("dns");
const { Resolver } = dns;

// ---------------------------------------------------------------------------
// Static hostname → IP mapping from environment variables
// ---------------------------------------------------------------------------
const _staticMap = new Map();

// DNS_FALLBACK_MAP="host1=1.2.3.4,host2=5.6.7.8"
if (process.env.DNS_FALLBACK_MAP) {
  for (const entry of process.env.DNS_FALLBACK_MAP.split(",")) {
    const [host, ip] = entry.trim().split("=");
    if (host && ip) _staticMap.set(host.trim().toLowerCase(), ip.trim());
  }
}
// DNS_FALLBACK_HOSTNAMES="host1,host2" + DNS_FALLBACK_IP="1.2.3.4"
if (process.env.DNS_FALLBACK_HOSTNAMES && process.env.DNS_FALLBACK_IP) {
  const ip = process.env.DNS_FALLBACK_IP.trim();
  for (const host of process.env.DNS_FALLBACK_HOSTNAMES.split(",")) {
    const h = host.trim().toLowerCase();
    if (h && !_staticMap.has(h)) _staticMap.set(h, ip);
  }
}

if (_staticMap.size > 0) {
  const entries = [..._staticMap.entries()]
    .map(([h, ip]) => `${h}->${ip}`)
    .join(", ");
  // eslint-disable-next-line no-console
  console.error(`[dns-fallback] Static overrides: ${entries}`);
}

// ---------------------------------------------------------------------------
// Public recursive resolvers (fallback when static map has no entry)
// ---------------------------------------------------------------------------
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
          // resolveNs returns hostnames, but setServers needs IP addresses.
          // Resolve each NS hostname to an IP first.
          let resolved = 0;
          const nsIPs = [];
          nsRecords.forEach((nsHost) => {
            publicResolver.resolve4(nsHost, (resolveErr, addrs) => {
              if (!resolveErr && addrs && addrs.length > 0) {
                nsIPs.push(...addrs);
              }
              resolved++;
              if (resolved === nsRecords.length) {
                if (nsIPs.length > 0) {
                  const authResolver = new Resolver();
                  authResolver.setServers(nsIPs);
                  return resolve(authResolver);
                }
                tryNext();
              }
            });
          });
          return;
        }
        tryNext();
      });
    }
    tryNext();
  });
}

/**
 * Resolve hostname via static map, public DNS, then authoritative NS as a
 * last resort.  Returns an array of IPv4 addresses or throws the original
 * error.
 */
async function _resolveWithFallback(hostname, origErr) {
  // 1. Static map — instant, no network needed
  const staticIP = _staticMap.get(hostname.toLowerCase());
  if (staticIP) return [staticIP];

  // 2. Public recursive resolvers
  const publicAddrs = await new Promise((resolve) => {
    publicResolver.resolve4(hostname, (err, addrs) => {
      if (!err && addrs && addrs.length > 0) return resolve(addrs);
      resolve(null);
    });
  });
  if (publicAddrs) return publicAddrs;

  // 3. Authoritative NS directly (bypasses negative-cache TTLs)
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
      // System resolver failed — try static map, public DNS, then authoritative NS
      _resolveWithFallback(hostname, err)
        .then((addresses) => {
          if (options && options.all) {
            return callback(
              null,
              addresses.map((a) => ({ address: a, family: 4 }))
            );
          }
          callback(null, addresses[0], 4);
        })
        .catch((fallbackErr) => callback(fallbackErr));
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
      // System resolver failed — try static map, public DNS, then authoritative NS
      const addresses = await _resolveWithFallback(hostname, err);

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
