import { defineConfig } from "astro/config";
import sitemap from "@astrojs/sitemap";

export default defineConfig({
  site: "https://getfuku.sh",
  base: "/",
  integrations: [sitemap()],
});
