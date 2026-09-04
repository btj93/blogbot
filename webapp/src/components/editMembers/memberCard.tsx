import {
  Card,
  CardBody,
  Divider,
  Image,
  Stack,
  Switch,
  Text,
} from '@chakra-ui/react'
import type * as CSS from 'csstype'
import React, { useMemo } from 'react'
import { useDispatch } from 'react-redux'
import { GroupInfos } from '../../constants'
import { updateMembers, useMember, useMemberImages } from '../../redux/member'

const MemberCardInner = ({
  name,
  borderColor,
}: {
  name: string
  borderColor: CSS.Property.BorderTopColor
}) => {
  const dispatch = useDispatch()

  const memberImages = useMemberImages()
  const member = useMember(name)

  const imageSrc = useMemo(() => {
    if (!member) return ''
    const image = memberImages[member.group]?.find(
      ({ name: imgName }) => imgName.replaceAll(' ', '') === member.name
    )
    return image?.img || GroupInfos[member.group].icon
  }, [member, memberImages])

  const onCardClick = () => {
    dispatch(updateMembers({ names: [name] }))
  }

  const subscribed = member?.subscribed ?? false
  const transform = `scale(${subscribed ? 0.98 : 1.0})`
  const transition = 'all 0.1s linear'

  const style: React.CSSProperties = {
    borderColor,
    WebkitTransition: transition,
    MozTransition: transition,
    msTransition: transition,
    OTransition: transition,
    transition,
    WebkitTransform: transform,
    MozTransform: transform,
    msTransform: transform,
    OTransform: transform,
    transform,
    boxShadow: subscribed ? `0px 0px 10px 3px ${borderColor}` : '',
  } as React.CSSProperties

  return (
    <Card style={style} onClick={onCardClick}>
      <CardBody>
        <Image
          width="100%"
          objectFit="cover"
          src={imageSrc}
          alt={name}
          borderRadius="lg"
          loading="lazy"
          sx={{ aspectRatio: '3/4' }}
        />
        <Stack mt="2" spacing="2">
          <Divider color="black" />
          <Text
            display="flex"
            justifyContent="space-between"
            fontSize="100%"
            alignItems="center"
          >
            {name}
            <Switch
              alignItems="center"
              isChecked={subscribed}
              onChange={onCardClick}
            />
          </Text>
        </Stack>
      </CardBody>
    </Card>
  )
}

export const MemberCard = React.memo(MemberCardInner)
