/**
 * Unified cluster connection state for global UI (banner, top bar).
 */

const AUTH_ERROR_HINTS = [
  'unauthorized',
  'forbidden',
  'expired',
  'token',
  'credentials',
  'authentication',
  'not authorized',
  'exec plugin',
  'get-token',
  'aws-iam-authenticator',
  'aws eks',
  'sso',
  'invalidclienttokenid',
  'no valid credential',
  'accessdenied',
  'authentication failed',
]

const NETWORK_ERROR_HINTS = [
  'connection refused',
  'connection reset',
  'no such host',
  'dial tcp',
  'i/o timeout',
  'timeout',
  'network is unreachable',
  'tls handshake',
]

export function classifyConnectionError(raw) {
  const message = String(raw || '').trim()
  if (!message) {
    return { kind: 'unknown', message: 'Cluster connection lost', guidance: '' }
  }

  const lower = message.toLowerCase()
  if (AUTH_ERROR_HINTS.some((hint) => lower.includes(hint))) {
    return {
      kind: 'auth',
      message,
      guidance: 'Cluster credentials may have expired. Refresh AWS or EKS login (for example aws sso login), then reconnect.',
    }
  }
  if (NETWORK_ERROR_HINTS.some((hint) => lower.includes(hint))) {
    return {
      kind: 'network',
      message,
      guidance: 'Check VPN, network access, and that the cluster API endpoint is reachable.',
    }
  }
  return { kind: 'unknown', message, guidance: 'Check kubeconfig, credentials, and network access.' }
}

export function isClusterDisconnected(cluster, clusterStatus) {
  const context = cluster?.selectedContext || cluster?.currentContext || ''
  if (!context) return false

  const syncError = String(cluster?.syncError || '').trim()
  if (syncError) return true

  if (!clusterStatus) return false

  const apiError = String(clusterStatus.error || '').trim()
  if (!clusterStatus.apiReachable && apiError) return true

  return false
}

export function connectionErrorMessage(cluster, clusterStatus) {
  const syncError = String(cluster?.syncError || '').trim()
  if (syncError) return classifyConnectionError(syncError).message

  const apiError = String(clusterStatus?.error || '').trim()
  if (apiError) return classifyConnectionError(apiError).message

  return classifyConnectionError('').message
}

export function connectionGuidance(cluster, clusterStatus, {
  autoRetryExhausted = false,
  retryAttempt = 0,
  maxRetries = 5,
} = {}) {
  const syncError = String(cluster?.syncError || '').trim()
  const apiError = String(clusterStatus?.error || '').trim()
  const classified = classifyConnectionError(syncError || apiError)

  if (autoRetryExhausted) {
    const tries = Math.max(retryAttempt, maxRetries)
    if (classified.kind === 'auth') {
      return `Klew tried reconnecting ${tries} times and could not authenticate. Refresh AWS or EKS credentials, then click Try again.`
    }
    return `Klew tried reconnecting ${tries} times and could not reach the cluster. ${classified.guidance} Then click Try again.`
  }

  if (classified.guidance) return classified.guidance
  return 'Klew will retry automatically.'
}

function contextLabel(cluster) {
  return cluster?.selectedContext || cluster?.currentContext || 'cluster'
}

export function activeContextLabel(cluster, { connecting = false, connectingTarget = null } = {}) {
  if (connecting && connectingTarget?.kind === 'context' && connectingTarget.name) {
    return connectingTarget.name
  }
  return contextLabel(cluster)
}

export function deriveConnectionState({
  cluster,
  syncing = false,
  connecting = false,
  connectingTarget = null,
  clusterStatus = null,
  statusLoading = false,
  retryAttempt = 0,
  retryInSec = 0,
  autoRetryExhausted = false,
  maxRetries = 5,
  dismissed = false,
}) {
  const context = cluster?.selectedContext || cluster?.currentContext || ''
  const label = activeContextLabel(cluster, { connecting, connectingTarget })
  const classified = classifyConnectionError(
    String(cluster?.syncError || '').trim() || String(clusterStatus?.error || '').trim(),
  )

  if (!context) {
    return { phase: 'idle', showBanner: false }
  }

  if (syncing || connecting || (statusLoading && !clusterStatus && !String(cluster?.syncError || '').trim())) {
    const switchingContext = connecting && connectingTarget?.kind === 'context' && connectingTarget.name
    const switchingNamespace = connecting && connectingTarget?.kind === 'namespace' && connectingTarget.name
    const message = switchingContext
      ? `Switching to ${label}…`
      : switchingNamespace
        ? `Switching to namespace ${connectingTarget.name}…`
        : (connecting ? `Switching to ${label}…` : 'Checking cluster connection…')

    if (dismissed) {
      return {
        phase: 'connecting',
        showBanner: false,
        title: `Connecting to ${label}`,
        message,
        tone: 'info',
      }
    }

    return {
      phase: 'connecting',
      showBanner: true,
      title: `Connecting to ${label}`,
      message,
      tone: 'info',
      showRetry: false,
      showDismiss: true,
    }
  }

  if (isClusterDisconnected(cluster, clusterStatus)) {
    const message = connectionErrorMessage(cluster, clusterStatus)
    const guidance = connectionGuidance(cluster, clusterStatus, {
      autoRetryExhausted,
      retryAttempt,
      maxRetries,
    })

    if (dismissed) {
      return {
        phase: 'disconnected',
        showBanner: false,
        title: `${label} disconnected`,
        message,
        tone: 'error',
      }
    }

    if (retryInSec > 0 && !autoRetryExhausted) {
      return {
        phase: 'retrying',
        showBanner: true,
        title: classified.kind === 'auth'
          ? `Reconnecting to ${label} — credentials may have expired`
          : `Reconnecting to ${label}`,
        message,
        detail: retryAttempt > 0
          ? `Attempt ${retryAttempt} of ${maxRetries} · retrying in ${retryInSec}s`
          : `Retrying in ${retryInSec}s`,
        tone: 'warn',
        showRetry: true,
        retryLabel: 'Reconnect now',
        showSettings: true,
        showDismiss: true,
      }
    }

    return {
      phase: 'disconnected',
      showBanner: true,
      title: autoRetryExhausted
        ? `Could not connect to ${label}`
        : (classified.kind === 'auth' ? `${label} disconnected — check credentials` : `${label} disconnected`),
      message,
      detail: autoRetryExhausted
        ? guidance
        : (classified.kind === 'auth'
          ? 'Klew will retry automatically. Refresh AWS or EKS login if retries keep failing.'
          : 'Klew will retry automatically.'),
      tone: 'error',
      showRetry: true,
      retryLabel: autoRetryExhausted ? 'Try again' : 'Reconnect now',
      showSettings: true,
      showDismiss: true,
    }
  }

  return { phase: 'connected', showBanner: false }
}
