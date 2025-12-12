```sh
bun create vite@latest client --template vanilla-ts
# I enabled Rolldown-Vite for this project

# Install marko and TailwindCSS using vite
bun install -D @marko/vite @marko/type-check 
bun install marko@next tailwindcss @tailwindcss/vite @marko/run

```

Create a `vite.config.ts` file with the following content:

```ts
import { defineConfig } from 'vite'
import tailwindcss from '@tailwindcss/vite'
import marko from "@marko/vite";

export default defineConfig({
  plugins: [
    tailwindcss(),
    marko(),
  ],
})
```