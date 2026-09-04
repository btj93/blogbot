import { ChakraProvider, extendTheme } from '@chakra-ui/react'
import React from 'react'
import ReactDOM from 'react-dom/client'
import { Provider } from 'react-redux'
import { EditMembers } from './components/editMembers'
import { GroupInfos } from './constants'
import './index.css'
import store from './redux/store'
import reportWebVitals from './reportWebVitals'

const fonts = `'ヒラギノ角ゴ Pro W3', 'Hiragino Kaku Gothic Pro', Osaka,
'メイリオ', Meiryo, 'ＭＳ Ｐゴシック', 'MS PGothic', sans-serif`

const theme = extendTheme({
  colors: Object.fromEntries(
    Object.entries(GroupInfos).map(([group, info]) => [group, info.color])
  ),
  fonts: {
    heading: fonts,
    body: fonts,
  },
})

const root = ReactDOM.createRoot(document.getElementById('root') as HTMLElement)
root.render(
  <React.StrictMode>
    <Provider store={store}>
      <ChakraProvider theme={theme}>
        <EditMembers />
      </ChakraProvider>
    </Provider>
  </React.StrictMode>
)

// If you want to start measuring performance in your app, pass a function
// to log results (for example: reportWebVitals(console.log))
// or send to an analytics endpoint. Learn more: https://bit.ly/CRA-vitals
reportWebVitals()
