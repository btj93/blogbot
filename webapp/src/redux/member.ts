import { useSelector } from 'react-redux'
import { type Member, type MemberImages } from '../types'

// Action types
export const UPDATE_MEMBERS = 'UPDATE_MEMBERS'
export const SET_MEMBERS = 'SET_MEMBERS'
export const SET_MEMBER_IMAGES = 'SET_MEMBER_IMAGES'

// Actions
export const updateMembers = ({
  names,
  subscribed,
}: {
  names: string[]
  subscribed?: boolean
}) => ({
  type: UPDATE_MEMBERS,
  names,
  subscribed,
})

export const setMembers = (members: Member[]) => ({
  type: SET_MEMBERS,
  members,
})

export const setMemberImages = (memberImages: MemberImages) => ({
  type: SET_MEMBER_IMAGES,
  memberImages,
})

interface MemberState {
  members: Member[]
  originalMembers: Member[]
  memberImages: MemberImages
}

// Initial states
const initialState: MemberState = {
  members: [],
  originalMembers: [],
  memberImages: {},
}

// Selectors
export const useMembers = (): Member[] =>
  useSelector((state: any) => state.member.members)
export const useOriginalMembers = (): Member[] =>
  useSelector((state: any) => state.member.originalMembers)
export const useMemberImages = (): MemberImages =>
  useSelector((state: any) => state.member.memberImages)
export const useMember = (name: string): Member | undefined =>
  useSelector(
    (state: any) =>
      (state.member.members as Member[]).find(m => m.name === name),
    (a?: Member, b?: Member) =>
      a?.subscribed === b?.subscribed && a?.name === b?.name
  )

// Reducers
export default function reducer(
  state = initialState,
  {
    type,
    names,
    members,
    memberImages,
    subscribed,
  }: {
    type: string
    names: string[]
    members: Member[]
    memberImages: MemberImages
    subscribed: boolean
  }
) {
  switch (type) {
    case UPDATE_MEMBERS:
      return {
        ...state,
        members: state.members.map(member =>
          names.includes(member.name)
            ? { ...member, subscribed: subscribed ?? !member.subscribed }
            : member
        ),
      }
    case SET_MEMBERS:
      return { ...state, members, originalMembers: members }
    case SET_MEMBER_IMAGES:
      return { ...state, memberImages }
    default:
      return state
  }
}
