import { useEffect, useState } from 'react'
import { GetAppInfo } from '../../wailsjs/go/main/App'

/** @returns {import('../../wailsjs/go/models').version.Info | null} */
export function useAppInfo() {
  const [info, setInfo] = useState(null)

  useEffect(() => {
    GetAppInfo?.().then(setInfo).catch(() => {})
  }, [])

  return info
}
