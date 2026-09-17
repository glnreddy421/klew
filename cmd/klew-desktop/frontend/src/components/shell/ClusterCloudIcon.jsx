/** Kubernetes cluster cloud — solid when online, struck when disconnected. */
export function ClusterCloudIcon({
  tone = 'ok',
  className = '',
  size = 14,
}) {
  const cls = [
    'cluster-cloud-icon',
    `tone-${tone}`,
    className,
  ].filter(Boolean).join(' ')

  return (
    <svg
      className={cls}
      viewBox="0 0 20 14"
      width={size}
      height={Math.round(size * 0.7)}
      aria-hidden="true"
    >
      <path
        d="M5.5 11.5h9a3.5 3.5 0 0 0 .6-6.95A4.5 4.5 0 0 0 4.2 3.8 3 3 0 0 0 5.5 11.5Z"
        fill="currentColor"
        opacity="0.92"
      />
      {(tone === 'crit' || tone === 'warn') && (
        <path
          d="M3 12.5 17 1.5"
          stroke="currentColor"
          strokeWidth="1.75"
          strokeLinecap="round"
        />
      )}
    </svg>
  )
}
