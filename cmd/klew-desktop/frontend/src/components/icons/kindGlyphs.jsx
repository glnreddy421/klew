/**
 * Klew's own Kubernetes kind icons: line art on a 24×24 grid, drawn with
 * currentColor so they follow the theme and status tones.
 *
 * Families share a base shape so kinds read as related at 12–16px:
 * workloads are boxes, networking is nodes and links, config is documents,
 * storage is cylinders and disks, access is people and badges, cluster
 * objects are frames and servers.
 */

const dot = (cx, cy) => <circle cx={cx} cy={cy} r="1" fill="currentColor" stroke="none" />

// Shared outlines.
const cube = (
  <>
    <path d="M12 3l8 4.5v9L12 21l-8-4.5v-9z" />
    <path d="M4 7.5l8 4.5 8-4.5M12 12v9" />
  </>
)
const cylinder = (
  <>
    <ellipse cx="12" cy="6" rx="7" ry="2.75" />
    <path d="M5 6v12c0 1.5 3.1 2.75 7 2.75s7-1.25 7-2.75V6" />
  </>
)
const doc = <path d="M6 3h8l4 4v14H6zM14 3v4h4" />
const shield = <path d="M12 3l7 3v5c0 4.5-3 8.3-7 10-4-1.7-7-5.5-7-10V6z" />
const badge = (
  <>
    <rect x="3" y="5" width="18" height="14" rx="2.5" />
    <circle cx="9" cy="11" r="2" />
    <path d="M6 16a3 3 0 0 1 6 0M14.5 10h4M14.5 14h3" />
  </>
)
const chain = (
  <path d="M10 14a3.5 3.5 0 0 0 5 0l3-3a3.5 3.5 0 0 0-5-5l-1 1M14 10a3.5 3.5 0 0 0-5 0l-3 3a3.5 3.5 0 0 0 5 5l1-1" />
)
const person = <path d="M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8zM4.5 21a7.5 7.5 0 0 1 15 0" />
const clusterFrame = <rect x="1.75" y="1.75" width="20.5" height="20.5" rx="4" strokeDasharray="2.5 2.5" />

const GENERIC = (
  <>
    <rect x="4" y="4" width="16" height="16" rx="3.5" />
    <path d="M12 8.5l3.5 3.5-3.5 3.5L8.5 12z" />
  </>
)

export const KIND_GLYPHS = {
  // Workloads
  Pod: cube,
  ReplicaSet: (
    <>
      <rect x="8" y="3" width="13" height="13" rx="2.5" />
      <path d="M16 21H5.5A2.5 2.5 0 0 1 3 18.5V8" />
    </>
  ),
  ReplicationController: (
    <>
      <rect x="8" y="3" width="13" height="13" rx="2.5" />
      <path d="M16 21H5.5A2.5 2.5 0 0 1 3 18.5V8M12 9.5h5" />
    </>
  ),
  Deployment: (
    <>
      <rect x="3" y="3" width="18" height="18" rx="3.5" />
      <path d="M16 12a4 4 0 1 1-1.2-2.85M16 7.75V10h-2.25" />
    </>
  ),
  StatefulSet: (
    <>
      <rect x="3" y="3.5" width="18" height="4.5" rx="1.5" />
      <rect x="3" y="9.75" width="18" height="4.5" rx="1.5" />
      <rect x="3" y="16" width="18" height="4.5" rx="1.5" />
      {dot(6.5, 5.75)}
      {dot(6.5, 12)}
      {dot(6.5, 18.25)}
    </>
  ),
  DaemonSet: (
    <>
      <rect x="3" y="3" width="7.5" height="7.5" rx="2" />
      <rect x="13.5" y="3" width="7.5" height="7.5" rx="2" />
      <rect x="3" y="13.5" width="7.5" height="7.5" rx="2" />
      <rect x="13.5" y="13.5" width="7.5" height="7.5" rx="2" />
    </>
  ),
  Job: (
    <>
      <rect x="3" y="3" width="18" height="18" rx="3.5" />
      <path d="M8 12.5l2.75 2.75L16 10" />
    </>
  ),
  CronJob: (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="M12 7v5l3.25 2" />
    </>
  ),
  ControllerRevision: (
    <>
      <rect x="3" y="3" width="18" height="18" rx="3.5" />
      <path d="M12 8v4l2.5 1.5M8 4v2M16 4v2" />
    </>
  ),
  HorizontalPodAutoscaler: (
    <>
      <rect x="8.5" y="8" width="7" height="8" rx="1.5" />
      <path d="M2.5 12h4M17.5 12h4M5 9.5L2.5 12 5 14.5M19 9.5l2.5 2.5-2.5 2.5" />
    </>
  ),
  PodDisruptionBudget: (
    <>
      <path d="M3 12a9 9 0 0 1 18 0z" />
      <path d="M12 12v6a2 2 0 0 0 4 0" />
    </>
  ),
  PriorityClass: <path d="M6 12l6-6 6 6M6 19l6-6 6 6" />,
  RuntimeClass: (
    <>
      <rect x="3" y="4" width="18" height="16" rx="2.5" />
      <path d="M7 9l3 3-3 3M12.5 15H17" />
    </>
  ),

  // Networking
  Service: (
    <>
      <circle cx="12" cy="5" r="2.25" />
      <circle cx="5" cy="19" r="2.25" />
      <circle cx="12" cy="19" r="2.25" />
      <circle cx="19" cy="19" r="2.25" />
      <path d="M12 7.25v9.5M11 7l-5 10M13 7l5 10" />
    </>
  ),
  Ingress: (
    <>
      <path d="M3 12h11M10.5 8.5L14 12l-3.5 3.5" />
      <path d="M14 3.5h4.5A2.5 2.5 0 0 1 21 6v12a2.5 2.5 0 0 1-2.5 2.5H14" />
    </>
  ),
  IngressClass: (
    <>
      <path d="M3 12h9M8.5 8.5L12 12l-3.5 3.5" />
      <rect x="14" y="4" width="7" height="16" rx="2" />
    </>
  ),
  Gateway: (
    <>
      <path d="M4 21V9a8 8 0 0 1 16 0v12" />
      <path d="M8 21v-9a4 4 0 0 1 8 0v9" />
    </>
  ),
  HTTPRoute: (
    <>
      <circle cx="5" cy="6" r="2.25" />
      <circle cx="19" cy="18" r="2.25" />
      <path d="M7.25 6H14a3 3 0 0 1 0 6h-4a3 3 0 0 0 0 6h6.75" />
    </>
  ),
  Endpoints: (
    <>
      <circle cx="12" cy="12" r="8.5" />
      <circle cx="12" cy="12" r="3.5" />
      {dot(12, 12)}
    </>
  ),
  EndpointSlice: (
    <>
      <circle cx="12" cy="12" r="8.5" />
      <path d="M12 12V3.5a8.5 8.5 0 0 1 8.5 8.5z" />
    </>
  ),
  NetworkPolicy: (
    <>
      <rect x="3" y="4.5" width="18" height="15" rx="2" />
      <path d="M3 9.5h18M3 14.5h18M9 4.5v5M15 9.5v5M9 14.5v5" />
    </>
  ),

  // Config
  ConfigMap: (
    <>
      {doc}
      <path d="M9 12h6M9 16h6" />
    </>
  ),
  Secret: (
    <>
      <rect x="5" y="10.5" width="14" height="10.5" rx="2.5" />
      <path d="M8.5 10.5V7.5a3.5 3.5 0 0 1 7 0v3" />
      {dot(12, 15.75)}
    </>
  ),

  // Storage
  PersistentVolume: (
    <>
      {cylinder}
      <path d="M5 12c0 1.5 3.1 2.75 7 2.75s7-1.25 7-2.75" />
    </>
  ),
  PersistentVolumeClaim: (
    <>
      <ellipse cx="9.5" cy="6" rx="6" ry="2.5" />
      <path d="M3.5 6v12c0 1.4 2.7 2.5 6 2.5s6-1.1 6-2.5V6" />
      <path d="M21.5 13h-4.5M19 10.75L16.75 13 19 15.25" />
    </>
  ),
  StorageClass: (
    <>
      <ellipse cx="12" cy="5.5" rx="8" ry="2.5" />
      <path d="M4 11.5c0 1.4 3.6 2.5 8 2.5s8-1.1 8-2.5M4 17.5c0 1.4 3.6 2.5 8 2.5s8-1.1 8-2.5" />
    </>
  ),
  Volume: (
    <>
      <rect x="3" y="7" width="18" height="10" rx="2.5" />
      <path d="M7 12h6" />
      {dot(17, 12)}
    </>
  ),
  VolumeAttachment: (
    <>
      <rect x="3" y="12" width="18" height="8" rx="2.5" />
      <path d="M12 3v6M9 6.5L12 9.5l3-3" />
      {dot(17, 16)}
    </>
  ),
  CSIDriver: (
    <>
      <rect x="6" y="6" width="12" height="12" rx="2" />
      <path d="M9 3v3M15 3v3M9 18v3M15 18v3M3 9h3M3 15h3M18 9h3M18 15h3" />
    </>
  ),

  // Access
  ServiceAccount: (
    <>
      <rect x="6" y="3.5" width="12" height="9" rx="2.5" />
      <path d="M12 1.5v2M4.5 21a7.5 7.5 0 0 1 15 0" />
      {dot(9.5, 8)}
      {dot(14.5, 8)}
    </>
  ),
  User: person,
  Group: (
    <>
      <path d="M9 11a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7zM2.5 20a6.5 6.5 0 0 1 13 0" />
      <path d="M15.5 4.3a3.5 3.5 0 0 1 0 6.4M18 13.6a6.5 6.5 0 0 1 3.5 6.4" />
    </>
  ),
  Role: badge,
  ClusterRole: (
    <>
      {clusterFrame}
      <g transform="translate(12 12) scale(0.78) translate(-12 -12)">{badge}</g>
    </>
  ),
  RoleBinding: chain,
  ClusterRoleBinding: (
    <>
      {clusterFrame}
      <g transform="translate(12 12) scale(0.8) translate(-12 -12)">{chain}</g>
    </>
  ),
  PodSecurityPolicy: (
    <>
      {shield}
      <path d="M9 12l2 2 4-4" />
    </>
  ),

  // Cluster
  Namespace: <rect x="3" y="3" width="18" height="18" rx="3.5" strokeDasharray="3 2.5" />,
  Node: (
    <>
      <rect x="5.5" y="2.5" width="13" height="19" rx="2.5" />
      <path d="M9 7h6M9 10.5h6" />
      {dot(12, 17)}
    </>
  ),
  ResourceQuota: (
    <>
      <path d="M3.5 18a8.5 8.5 0 1 1 17 0" />
      <path d="M12 18l4.25-5.5" />
      {dot(12, 18)}
    </>
  ),
  LimitRange: <path d="M3 12h18M3 8v8M21 8v8M7.5 10v4M12 9v6M16.5 10v4" />,
  Lease: <path d="M7 3h10M7 21h10M8 3c0 5 8 5 8 9s-8 4-8 9M16 3c0 5-8 5-8 9s8 4 8 9" />,
  Event: <path d="M13 2.5L4.5 14H11l-1 7.5L18.5 10H12z" />,
  CustomResourceDefinition: (
    <path d="M8 3.5c-2 0-3 1-3 3V9c0 1.5-.75 3-2 3 1.25 0 2 1.5 2 3v2.5c0 2 1 3 3 3M16 3.5c2 0 3 1 3 3V9c0 1.5.75 3 2 3-1.25 0-2 1.5-2 3v2.5c0 2-1 3-3 3" />
  ),
  APIService: (
    <>
      <rect x="3" y="5" width="18" height="14" rx="2.5" />
      <path d="M8 10l-2 2 2 2M16 10l2 2-2 2M13 9l-2 6" />
    </>
  ),
  ValidatingWebhookConfiguration: (
    <>
      <path d="M17 4v6.5a5 5 0 0 1-10 0V9M4.5 11.5L7 9l2.5 2.5" />
      <path d="M9 20h6" />
    </>
  ),
  MutatingWebhookConfiguration: (
    <>
      <path d="M17 4v6.5a5 5 0 0 1-10 0V9M4.5 11.5L7 9l2.5 2.5" />
      <path d="M9 20h6M12 17v6" />
    </>
  ),
  HelmRelease: (
    <>
      <circle cx="12" cy="12" r="6" />
      <circle cx="12" cy="12" r="2" />
      <path d="M12 2.5v3.5M12 18v3.5M2.5 12H6M18 12h3.5M5.3 5.3l2.5 2.5M16.2 16.2l2.5 2.5M5.3 18.7l2.5-2.5M16.2 7.8l2.5-2.5" />
    </>
  ),
}

/** Icons for live stream sources (LOG, EVENT, …). */
export const STREAM_GLYPHS = {
  LOG: <path d="M4 6h16M4 10h12M4 14h16M4 18h9" />,
  EVENT: KIND_GLYPHS.Event,
  KLEW: (
    <>
      <circle cx="10.5" cy="10.5" r="6.5" />
      <path d="M15.5 15.5L21 21" />
    </>
  ),
  METRIC: <path d="M3 20.5h18M6 20V11M11 20V5M16 20v-7" />,
  OBJ: GENERIC,
  SYS: (
    <>
      <circle cx="12" cy="12" r="3" />
      <path d="M12 2.5v3M12 18.5v3M2.5 12h3M18.5 12h3M5.3 5.3l2.1 2.1M16.6 16.6l2.1 2.1M5.3 18.7l2.1-2.1M16.6 7.4l2.1-2.1" />
    </>
  ),
}

export const GENERIC_GLYPH = GENERIC

/** Glyph for a normalized kind; unknown and custom kinds get the generic one. */
export function glyphForKind(normalizedKind) {
  return KIND_GLYPHS[normalizedKind] || GENERIC
}

export function hasOwnGlyph(normalizedKind) {
  return Object.prototype.hasOwnProperty.call(KIND_GLYPHS, normalizedKind)
}

/** Stroke attributes shared by every glyph so they render identically anywhere. */
export const GLYPH_STROKE = {
  fill: 'none',
  stroke: 'currentColor',
  strokeWidth: 1.6,
  strokeLinecap: 'round',
  strokeLinejoin: 'round',
}

/** Draws a kind glyph inside another SVG, e.g. a graph node. `size` is in user units. */
export function KindGlyph({ glyph, x = 0, y = 0, size = 24, className = '' }) {
  const s = size / 24
  return (
    <g className={className} transform={`translate(${x} ${y}) scale(${s})`} {...GLYPH_STROKE}>
      {glyph}
    </g>
  )
}
