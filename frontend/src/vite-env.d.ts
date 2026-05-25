/// <reference types="vite/client" />

import 'axios';

declare module 'axios' {
  export interface AxiosRequestConfig<D = any> {
    skipUnauthorizedRedirect?: boolean;
  }
}
