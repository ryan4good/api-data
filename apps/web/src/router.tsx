import { createBrowserRouter } from 'react-router-dom'
import { routes } from './routes'

const basename = import.meta.env.BASE_URL.replace(/\/$/, '') || '/'

export const router = createBrowserRouter(routes, { basename })
