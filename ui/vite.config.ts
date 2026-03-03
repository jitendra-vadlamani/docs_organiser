import * as reactPlugin from 'vite-plugin-react'
import type { UserConfig } from 'vite'

const config: UserConfig = {
  jsx: 'react',
  plugins: [reactPlugin],
  proxy: {
    '/v1': {
      target: 'http://localhost:8090',
      changeOrigin: true
    }
  }
}

export default config
