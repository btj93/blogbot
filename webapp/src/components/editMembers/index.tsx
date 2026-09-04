import { Alert, AlertIcon, Image, Text } from '@chakra-ui/react'
import {
  MainButton,
  type MainButtonProps,
} from '@vkruglikov/react-telegram-web-app'
import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useDispatch } from 'react-redux'
import { ScaleLoader } from 'react-spinners'
import {
  acquireLock,
  ApiError,
  fetchMemberImages,
  fetchMembers,
  saveSubscriptions,
} from '../../apiClient'
import { GroupInfos } from '../../constants'
import {
  setMemberImages,
  setMembers,
  useMembers,
  useOriginalMembers,
} from '../../redux/member'
import { AnimatedTab, type AnimatedTabConfig } from './animatedTab'
import { MembersAccordion } from './membersAccordion'

export const EditMembers = () => {
  const [isLoading, setIsLoading] = useState<boolean>(true)
  const [isSaving, setIsSaving] = useState<boolean>(false)
  const [error, setError] = useState<string | null>(null)
  const [chatName, setChatName] = useState<string>('')
  const [lockId, setLockId] = useState<string | null>(null)
  const [lockHolder, setLockHolder] = useState<string | null>(null)
  const dispatch = useDispatch()

  useEffect(() => {
    const tg = (window as any).Telegram?.WebApp
    tg?.ready()
    tg?.expand()

    const loadData = async () => {
      try {
        // Acquire lock first.
        const lockResult = await acquireLock()
        if (lockResult.status === 'locked') {
          setLockHolder(lockResult.holder)
          setIsLoading(false)
          return
        }

        setLockId(lockResult.lockId)

        const data = await fetchMembers()
        dispatch(setMembers(data.members))
        setChatName(data.chat_name)
      } catch (err) {
        if (err instanceof ApiError) {
          if (err.message === 'not_in_telegram') {
            setError('請從 Telegram 開啟此應用程式')
          } else if (err.status === 401) {
            setError('認證失敗，請重新開啟應用程式')
          } else if (err.status === 403) {
            setError('管理員權限確認失敗，請稍後再試')
          } else {
            setError('載入失敗，請重試')
          }
        } else {
          setError('網路錯誤，請檢查連線')
        }
        return
      } finally {
        setIsLoading(false)
      }
    }

    loadData()

    // Load images independently — don't block the member list.
    fetchMemberImages().then(images => dispatch(setMemberImages(images)))
  }, [dispatch])

  const members = useMembers()
  const originalMembers = useOriginalMembers()
  const membersRef = useRef(members)
  const originalMembersRef = useRef(originalMembers)
  membersRef.current = members
  originalMembersRef.current = originalMembers

  // Group members by group name, sorted by GroupInfos key order.
  // Memoized because group membership never changes — only subscribed toggles.
  const membersByGroupEntries = useMemo(() => {
    const grouped: Record<string, typeof members> = {}
    for (const m of members) {
      ;(grouped[m.group] ??= []).push(m)
    }
    const groupOrder = Object.keys(GroupInfos)
    return Object.entries(grouped).sort(
      ([a], [b]) => groupOrder.indexOf(a) - groupOrder.indexOf(b)
    )
  }, [members])

  const hasUnsavedChanges = useCallback(() => {
    const origMap = new Map(originalMembersRef.current.map(m => [m.id, m]))
    return membersRef.current.some(m => {
      const orig = origMap.get(m.id)
      return orig && orig.subscribed !== m.subscribed
    })
  }, [])

  // Unsaved changes guard — intercept Telegram back button.
  useEffect(() => {
    const tg = (window as any).Telegram?.WebApp
    if (!tg) return

    tg.BackButton?.show()

    const onBackButtonClicked = () => {
      if (hasUnsavedChanges()) {
        tg.showConfirm(
          '尚未儲存的變更將會遺失，確定關閉？',
          (confirmed: boolean) => {
            if (confirmed) tg.close()
          }
        )
      } else {
        tg.close()
      }
    }

    tg.BackButton?.onClick(onBackButtonClicked)
    return () => tg.BackButton?.offClick(onBackButtonClicked)
  }, [hasUnsavedChanges])

  useEffect(() => {
    if (!lockHolder) return

    const interval = setInterval(() => {
      void (async () => {
        try {
          const result = await acquireLock()
          if (result.status === 'acquired') {
            setLockId(result.lockId)
            setLockHolder(null)
            setIsLoading(true)

            try {
              const data = await fetchMembers()
              dispatch(setMembers(data.members))
              setChatName(data.chat_name)
            } catch {
              setError('載入失敗，請重試')
            } finally {
              setIsLoading(false)
            }
          } else {
            setLockHolder(result.holder)
          }
        } catch {
          // acquireLock failed — silently retry on next interval.
        }
      })()
    }, 30000)

    return () => clearInterval(interval)
  }, [lockHolder, dispatch])

  const getChanges = () => {
    const origMap = new Map(originalMembers.map(m => [m.id, m]))
    return members
      .filter(m => {
        const orig = origMap.get(m.id)
        return orig && orig.subscribed !== m.subscribed
      })
      .map(m => ({ member_id: m.id, subscribed: m.subscribed }))
  }

  const onClickMainButton = async () => {
    const changes = getChanges()

    if (changes.length === 0) {
      if (lockId) {
        try {
          await saveSubscriptions(lockId, [])
        } catch {
          /* best effort — lock will expire naturally */
        }
      }
      ;(window as any).Telegram.WebApp.close()
      return
    }

    if (!lockId) {
      setError('無法儲存：未取得編輯鎖')
      return
    }

    setIsSaving(true)
    try {
      await saveSubscriptions(lockId, changes)
      ;(window as any).Telegram.WebApp.close()
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        setError(`${err.message} 正在編輯中，儲存失敗，請關閉此視窗`)
      } else {
        setError('儲存失敗，請重試')
      }
      setIsSaving(false)
    }
  }

  const mainButton: MainButtonProps = {
    text: isSaving ? 'SAVING...' : 'SAVE',
    onClick: () => {
      void onClickMainButton()
    },
    disable: isSaving,
  }

  const tabConfigs: AnimatedTabConfig[] = membersByGroupEntries.map(
    ([group, members]) => ({
      key: group,
      tabHeader: (
        <Text
          display="flex"
          alignItems="center"
          justifyContent="center"
          alignSelf="auto"
        >
          <Image
            height="min(5vw, 26px)"
            src={GroupInfos[group]?.icon || ''}
            alt={group}
            margin="0px 10px 0px 0px"
          />
          {group}
        </Text>
      ),
      tabContent: <MembersAccordion members={members || []} key={group} />,
      activeColor: `${group}-500`,
    })
  )

  if (lockHolder) {
    return (
      <Alert
        status="warning"
        variant="subtle"
        flexDirection="column"
        alignItems="center"
        justifyContent="center"
        textAlign="center"
        height="100vh"
      >
        <AlertIcon boxSize="40px" mr={0} />
        <Text mt={4} fontSize="lg">
          {lockHolder} 正在編輯中
        </Text>
        <Text mt={2} fontSize="sm" color="gray.500">
          每 30 秒自動重試...
        </Text>
      </Alert>
    )
  }

  if (error) {
    return (
      <Alert
        status="error"
        variant="subtle"
        flexDirection="column"
        alignItems="center"
        justifyContent="center"
        textAlign="center"
        height="100vh"
      >
        <AlertIcon boxSize="40px" mr={0} />
        <Text mt={4} fontSize="lg">
          {error}
        </Text>
      </Alert>
    )
  }

  return (
    <>
      <div className="scale-loader-div">
        <div>
          <ScaleLoader
            className={`scale-loader ${
              isLoading ? 'visible' : 'hidden-transition'
            }`}
            height="60%"
            width="8%"
          />
        </div>
      </div>
      <div
        className={`main-page ${isLoading ? 'hidden' : 'visible-transition'}`}
      >
        {chatName && (
          <Text textAlign="center" fontSize="sm" color="gray.500" py={1}>
            正在編輯: {chatName}
          </Text>
        )}
        <AnimatedTab configs={tabConfigs} />
        <div>
          <MainButton {...mainButton} />
        </div>
      </div>
    </>
  )
}
