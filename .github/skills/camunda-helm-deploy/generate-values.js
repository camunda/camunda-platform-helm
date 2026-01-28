#!/usr/bin/env node
/**
 * Camunda Platform Helm Deployment Helper Script
 * 
 * This script can be used by the agent to generate values.yaml
 * Usage: node generate-values.js "<user request>"
 */

const YAML = require('yaml');
const knowledge = require('./knowledge.json');

// ============================================================================
// INTENT PARSER
// ============================================================================

function parseIntent(userMessage) {
  const msg = userMessage.toLowerCase();
  
  return {
    action: 'deploy',
    namespace: extractNamespace(msg) || 'camunda',
    hostname: extractHostname(msg),
    ingress: msg.includes('ingress') || extractHostname(msg) !== null,
    basicAuth: msg.includes('basic auth') || msg.includes('authentication') || msg.includes('auth'),
    tls: !msg.includes('no tls') && !msg.includes('without tls'),
    environment: msg.includes('production') || msg.includes('prod') ? 'production' : 'development',
    components: extractComponents(msg)
  };
}

function extractNamespace(msg) {
  const patterns = [
    /namespace[:\s]+([a-z0-9-]+)/i,
    /\bns[:\s]+([a-z0-9-]+)/i,
    /in\s+([a-z0-9-]+)\s+namespace/i
  ];
  
  for (const pattern of patterns) {
    const match = msg.match(pattern);
    if (match) return match[1];
  }
  return null;
}

function extractHostname(msg) {
  const match = msg.match(/(?:at|host|domain|on)\s+([a-z0-9][a-z0-9.-]+\.[a-z]{2,})/i);
  if (match) return match[1];
  
  const hostnameMatch = msg.match(/([a-z0-9][a-z0-9-]*\.(?:example\.com|[a-z0-9-]+\.[a-z]{2,}))/i);
  if (hostnameMatch) return hostnameMatch[1];
  
  return null;
}

function extractComponents(msg) {
  const allComponents = knowledge.components;
  const mentioned = allComponents.filter(c => msg.includes(c.toLowerCase()));
  
  const excluded = [];
  if (msg.includes('without optimize') || msg.includes('no optimize')) {
    excluded.push('optimize');
  }
  if (msg.includes('without modeler') || msg.includes('no modeler')) {
    excluded.push('webModeler');
  }
  
  if (mentioned.length > 0) {
    return mentioned.filter(c => !excluded.includes(c));
  }
  
  return allComponents.filter(c => !excluded.includes(c));
}

// ============================================================================
// VALUES GENERATOR
// ============================================================================

function generateValues(intent) {
  const envDefaults = knowledge.defaults[intent.environment];
  const ingressConfig = knowledge.ingress.controllers.nginx;
  
  const values = {
    global: {
      ingress: {
        enabled: intent.ingress,
        className: ingressConfig.className,
        host: intent.hostname || 'camunda.example.com',
        annotations: { ...ingressConfig.annotations },
        tls: {
          enabled: intent.tls,
          secretName: knowledge.ingress.tls.secretName
        }
      }
    }
  };
  
  if (intent.basicAuth && intent.ingress) {
    Object.assign(values.global.ingress.annotations, ingressConfig.basicAuth);
  }
  
  values.zeebe = {
    clusterSize: envDefaults.zeebe.clusterSize,
    partitionCount: envDefaults.zeebe.partitionCount,
    replicationFactor: envDefaults.zeebe.replicationFactor,
    resources: envDefaults.zeebe.resources
  };
  
  values.operate = { resources: envDefaults.operate.resources };
  values.tasklist = { resources: envDefaults.tasklist.resources };
  
  values.elasticsearch = {
    replicas: envDefaults.elasticsearch.replicas,
    minimumMasterNodes: envDefaults.elasticsearch.minimumMasterNodes,
    resources: envDefaults.elasticsearch.resources
  };
  
  const disabledComponents = knowledge.components.filter(c => !intent.components.includes(c));
  for (const comp of disabledComponents) {
    values[comp] = { enabled: false };
  }
  
  return values;
}

// ============================================================================
// COMMAND GENERATOR
// ============================================================================

function generateCommands(intent) {
  const commands = [];
  const chart = knowledge.chart;
  
  commands.push({
    description: 'Add Camunda Helm repository',
    command: `helm repo add camunda ${chart.repository}\nhelm repo update`
  });
  
  commands.push({
    description: 'Create namespace',
    command: `kubectl create namespace ${intent.namespace} --dry-run=client -o yaml | kubectl apply -f -`
  });
  
  if (intent.basicAuth) {
    commands.push({
      description: 'Create basic auth secret',
      command: `htpasswd -cb auth USERNAME PASSWORD
kubectl create secret generic camunda-basic-auth --from-file=auth -n ${intent.namespace}
rm auth`
    });
  }
  
  if (intent.tls && intent.ingress) {
    commands.push({
      description: 'Create TLS secret (if not using cert-manager)',
      command: `kubectl create secret tls ${knowledge.ingress.tls.secretName} \\
  --cert=path/to/tls.crt \\
  --key=path/to/tls.key \\
  -n ${intent.namespace}`
    });
  }
  
  commands.push({
    description: 'Install Camunda Platform',
    command: `helm upgrade --install camunda camunda/${chart.name} \\
  --namespace ${intent.namespace} \\
  --version ${chart.version} \\
  -f values.yaml \\
  --wait`
  });
  
  commands.push({
    description: 'Verify deployment',
    command: `kubectl get pods -n ${intent.namespace} -w`
  });
  
  return commands;
}

// ============================================================================
// OUTPUT
// ============================================================================

function formatOutput(intent, values, commands) {
  const output = {
    intent,
    values: YAML.stringify(values),
    commands
  };
  return JSON.stringify(output, null, 2);
}

// ============================================================================
// MAIN
// ============================================================================

if (require.main === module) {
  const userMessage = process.argv.slice(2).join(' ') || 
    'Deploy Camunda with ingress and basic auth at demo.camunda.io';
  
  const intent = parseIntent(userMessage);
  const values = generateValues(intent);
  const commands = generateCommands(intent);
  
  console.log(formatOutput(intent, values, commands));
}

module.exports = { parseIntent, generateValues, generateCommands };
