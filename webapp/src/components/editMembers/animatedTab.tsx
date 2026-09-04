import React, { useState } from 'react'
import { useSwipeable } from 'react-swipeable'

export interface AnimatedTabConfig {
  key: string
  tabHeader: JSX.Element
  tabContent: JSX.Element
  activeColor: string
}

export const AnimatedTab = ({ configs }: { configs: AnimatedTabConfig[] }) => {
  const [activeIndex, setActiveIndex] = useState<number>(0)
  const [mountedTabs, setMountedTabs] = useState<Set<number>>(new Set([0]))

  const numberOfTabs = configs.length

  const activateTab = (index: number) => {
    setActiveIndex(index)
    setMountedTabs(prev => (prev.has(index) ? prev : new Set(prev).add(index)))
  }

  const swipeHandlers = useSwipeable({
    onSwipedLeft: () =>
      activateTab(activeIndex === numberOfTabs - 1 ? 0 : activeIndex + 1),
    onSwipedRight: () => activateTab((activeIndex || numberOfTabs) - 1),
    preventScrollOnSwipe: true,
    trackMouse: true,
    trackTouch: true,
  })

  const onTabClick = (
    _event: React.MouseEvent<HTMLDivElement, globalThis.MouseEvent>,
    index: number
  ) => {
    if (activeIndex === index) {
      return
    }
    activateTab(index)
  }

  const getTabContentStyle = (i: number): React.CSSProperties => {
    const transform = `translate3d(${100 * (i - activeIndex)}%, 0, 0)`
    const transition = 'transform cubic-bezier(0.74, -0.21, 0.29, 1.3) 0.3s'
    return {
      position: 'absolute',
      top: 0,
      left: 0,
      WebkitTransform: transform,
      MozTransform: transform,
      msTransform: transform,
      OTransform: transform,
      transform,
      width: '100%',
      WebkitTransition: transition,
      MozTransition: transition,
      msTransition: transition,
      OTransition: transition,
      transition,
      height: '100%',
      overflowY: 'auto',
      WebkitOverflowScrolling: 'touch',
      borderRadius: '25px',
    } as React.CSSProperties
  }

  const tabTransition =
    'all cubic-bezier(0.74, -0.21, 0.29, 1.3) 0.3s, color 0.15s, background-color 0.3s'
  const tabTranslation = (i: number) => `translate3d(${100 * i}%, 0, 0)`

  return (
    <>
      <div className="animated-tab-wrapper">
        <style
          dangerouslySetInnerHTML={{
            __html: `.animated-tab-switch:after {
								content: '';
								position: absolute;
								width: ${100 / numberOfTabs}%;
								top: 0;
                left: 0;
                -webkit-transition: ${tabTransition};
                -moz-transition: ${tabTransition};
                -ms-transition: ${tabTransition};
                -o-transition: ${tabTransition};
								transition: ${tabTransition};
								border-radius: 27.5px;
								box-shadow: 0 2px 15px 0 rgba(0, 0, 0, 0.1);
								background-color: var(--chakra-colors-${configs[activeIndex]?.activeColor});
								height: 100%;
								z-index: 0;
						}

						${configs
              .map(
                (_, i) =>
                  `.animated-tab-switch.active-${i}:after {
                    -webkit-transform: ${tabTranslation(i)};
                    -moz-transform:  ${tabTranslation(i)};
                    -ms-transform:  ${tabTranslation(i)};
                    -o-transform:  ${tabTranslation(i)};
                    transform:  ${tabTranslation(i)};
							}`
              )
              .join('\n')}
						`,
          }}
        ></style>
        <div
          className={`animated-tab-switch active-${activeIndex} text-center`}
        >
          {configs.map((config, i) => (
            <div
              key={config.key}
              className={`animated-tab ${activeIndex === i ? 'active' : ''}`}
              onClick={event => onTabClick(event, i)}
            >
              {config.tabHeader}
            </div>
          ))}
        </div>
      </div>
      <div className="animated-tab-contents" {...swipeHandlers}>
        {configs.map((config, i) => (
          <div key={i} style={getTabContentStyle(i)}>
            {mountedTabs.has(i) ? config.tabContent : null}
          </div>
        ))}
      </div>
    </>
  )
}
