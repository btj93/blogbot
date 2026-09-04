export interface Member {
  id: number
  name: string
  subscribed: boolean
  group: string
  generation: string
}

interface MemberImage {
  name: string
  img: string
}

export type MemberImages = Record<string, MemberImage[]>

export interface GroupInfo {
  icon: string
  color: Record<string, string>
}
