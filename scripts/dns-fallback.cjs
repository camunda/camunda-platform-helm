/**
 * DNS Fallback Preload Script
 *
 * Loaded via NODE_OPTIONS="--require ./dns-fallback.cjs" when the system DNS
 * resolver has a stale NXDOMAIN cached for a hostname that public DNS can
 * already resolve.
 *
 * Monkey-patches dns.lookup (used by http/https/Playwright) to fall back to
 * Cloudflare (1.1.1.1) + Google (8.8.8.8) DNS when the system resolver
 * returns ENOTFOUND.  This avoids needing sudo or /etc/hosts edits.
 */
"use strict";

const dns = require("dns");
const { Resolver } = dns;

const resolver = new Resolver();
resolver.setServers(["1.1.1.1", "8.8.8.8", "9.9.9.9"]);

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
        callback(null, addresses[0], 4);
      });
    } else {
      callback(err, address, family);
    }
  });
};
