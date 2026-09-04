import {
  Accordion,
  AccordionButton,
  AccordionIcon,
  AccordionItem,
  AccordionPanel,
  Badge,
  Box,
  Button,
  ButtonGroup,
  SimpleGrid,
} from '@chakra-ui/react'
import React, { useMemo, useState } from 'react'
import { useDispatch } from 'react-redux'
import { updateMembers } from '../../redux/member'
import { type Member } from '../../types'
import { MemberCard } from './memberCard'

export const MembersAccordion = ({ members }: { members: Member[] }) => {
  const dispatch = useDispatch()

  const defaultExpandedIndex = [0]

  const [expandedIndex, setExpandedIndex] =
    useState<number[]>(defaultExpandedIndex)

  const toggleAccordionIndex = (index: number) => {
    if (expandedIndex.includes(index)) {
      setExpandedIndex(expandedIndex.filter(element => element !== index))
    } else {
      setExpandedIndex([...expandedIndex, index])
    }
  }

  const membersByGeneration = useMemo(() => {
    const grouped: Record<string, Member[]> = {}
    for (const m of members) {
      ;(grouped[m.generation] ??= []).push(m)
    }
    return grouped
  }, [members])

  const onClickAccordionButton = (
    event: React.MouseEvent<HTMLButtonElement, MouseEvent>,
    index: number
  ) => {
    const target = event.target as Element
    if (
      !(
        target.classList.contains('sub-button') ||
        target.classList.contains('unsub-button')
      )
    ) {
      toggleAccordionIndex(index)
    }
  }

  const subAll = (members: Member[]) => {
    dispatch(
      updateMembers({
        names: members.map(({ name }) => name),
        subscribed: true,
      })
    )
  }

  const unSubAll = (members: Member[]) => {
    dispatch(
      updateMembers({
        names: members.map(({ name }) => name),
        subscribed: false,
      })
    )
  }

  const getNumberOfSubedMembers = (members: Member[]) =>
    members.filter(member => member.subscribed).length

  const subButtonTransform = 'scale(0.98)'
  const subButtonTransition = 'all 0.2s cubic-bezier(.08,.52,.52,1)'
  const subButtonBoxShadow =
    '0 0 1px 2px rgba(88, 144, 255, .75), 0 1px 1px rgba(0, 0, 0, .15)'

  const subButtonStyle: React.CSSProperties = {
    WebkitTransition: subButtonTransition,
    MozTransition: subButtonTransition,
    msTransition: subButtonTransition,
    OTransition: subButtonTransition,
    transition: subButtonTransition,
  }

  return (
    <>
      <Accordion allowMultiple={true} index={expandedIndex}>
        {Object.entries(membersByGeneration).map(([gen, members], i) => (
          <AccordionItem key={gen} borderRadius="25px 25px 0px 0px">
            <h2>
              <AccordionButton
                onClick={event => onClickAccordionButton(event, i)}
              >
                <Box as="span" flex="1" textAlign="left">
                  {gen}
                  <Badge
                    variant={
                      getNumberOfSubedMembers(members) === members.length
                        ? 'solid'
                        : 'outline'
                    }
                    colorScheme={
                      getNumberOfSubedMembers(members) > 0 ? 'green' : 'red'
                    }
                    margin="0px 0px 0px max(1%, 8px)"
                    verticalAlign="center"
                    fontSize="1.0em"
                    rounded="md"
                  >
                    {`${getNumberOfSubedMembers(members)}/${members.length}`}
                  </Badge>
                </Box>
                <ButtonGroup variant="outline" spacing="1.5vw" size="sm">
                  <Button
                    colorScheme="blue"
                    style={subButtonStyle}
                    variant="solid"
                    onClick={() => subAll(members)}
                    className="sub-button"
                    _active={{
                      WebkitTransform: subButtonTransform,
                      MozTransform: subButtonTransform,
                      msTransform: subButtonTransform,
                      OTransform: subButtonTransform,
                      transform: subButtonTransform,
                    }}
                    _focus={{ boxShadow: subButtonBoxShadow }}
                  >
                    Sub All
                  </Button>
                  <Button
                    colorScheme="red"
                    style={subButtonStyle}
                    onClick={() => unSubAll(members)}
                    className="unsub-button"
                    _active={{
                      WebkitTransform: subButtonTransform,
                      MozTransform: subButtonTransform,
                      msTransform: subButtonTransform,
                      OTransform: subButtonTransform,
                      transform: subButtonTransform,
                    }}
                    _focus={{ boxShadow: subButtonBoxShadow }}
                  >
                    Unsub All
                  </Button>
                </ButtonGroup>
                <AccordionIcon margin="0px 0px 0px 2%" />
              </AccordionButton>
            </h2>
            <AccordionPanel pb={4}>
              <SimpleGrid columns={[2, null, 4]} spacing="8px">
                {members.map(({ name, group }) => (
                  <MemberCard
                    key={name}
                    name={name}
                    borderColor={`var(--chakra-colors-${group}-300)`}
                  />
                ))}
              </SimpleGrid>
            </AccordionPanel>
          </AccordionItem>
        ))}
      </Accordion>
    </>
  )
}
