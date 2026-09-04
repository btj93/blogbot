import { type Member, type MemberImages } from './types'

const API_BASE = process.env.REACT_APP_API_BASE_URL || ''

export class ApiError extends Error {
  status: number
  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

export const fetchMembers = async (): Promise<{
  chat_name: string
  members: Member[]
}> => {
  const initData = (window as any).Telegram?.WebApp?.initData || ''

  const res = await fetch(`${API_BASE}/tg/blogbot/api/v1/members`, {
    headers: { 'X-Telegram-Init-Data': initData },
    signal: AbortSignal.timeout(10000),
  })

  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new ApiError(res.status, body.error || 'request failed')
  }

  return await res.json()
}

export type LockResult =
  | { status: 'acquired'; lockId: string }
  | { status: 'locked'; holder: string }

export const acquireLock = async (): Promise<LockResult> => {
  const initData = (window as any).Telegram?.WebApp?.initData || ''

  const res = await fetch(`${API_BASE}/tg/blogbot/api/v1/lock`, {
    method: 'POST',
    headers: { 'X-Telegram-Init-Data': initData },
    signal: AbortSignal.timeout(10000),
  })

  if (res.status === 409) {
    const body = await res.json()
    return { status: 'locked', holder: body.holder }
  }

  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new ApiError(res.status, body.error || 'request failed')
  }

  const body = await res.json()
  return { status: 'acquired', lockId: body.lock_id }
}

export const saveSubscriptions = async (
  lockId: string,
  changes: Array<{ member_id: number; subscribed: boolean }>
): Promise<void> => {
  const initData = (window as any).Telegram?.WebApp?.initData || ''

  const res = await fetch(`${API_BASE}/tg/blogbot/api/v1/subscriptions`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      'X-Telegram-Init-Data': initData,
    },
    body: JSON.stringify({ lock_id: lockId, changes }),
    signal: AbortSignal.timeout(10000),
  })

  if (res.status === 409) {
    const body = await res.json()
    throw new ApiError(res.status, body.holder || 'locked')
  }

  if (!res.ok) {
    const body = await res.json().catch(() => ({}))
    throw new ApiError(res.status, body.error || 'request failed')
  }
}

// Image fetching — proxied through the Go backend.

export const fetchMemberImages = async (): Promise<MemberImages> => {
  try {
    const res = await fetch(`${API_BASE}/tg/blogbot/api/v1/member-images`, {
      signal: AbortSignal.timeout(15000),
    })

    if (res.ok) {
      return await res.json()
    }
  } catch {
    // Image fetch failed — fall back to group icons.
  }

  return {
    乃木坂46: [],
    櫻坂46: [],
    日向坂46: [],
  }
}
