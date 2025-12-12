import { defineConfig } from "vite";
import tailwindcss from "@tailwindcss/vite";
// import marko from "@marko/vite"; // We use marko-run instead
import marko from "@marko/run/vite";

export default defineConfig({
  plugins: [
    tailwindcss(),
    marko({
      //   routesDir: "src/pages",
    }),
  ],
});
