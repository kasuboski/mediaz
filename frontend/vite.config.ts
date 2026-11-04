import { configDefaults, defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react-swc";
import path from "node:path";
import { componentTagger } from "lovable-tagger";

export default defineConfig(({ mode }) => {
	const apiHost = mode === "production" ? "" : "http://localhost:8080";

	return {
		test: {
			exclude: [...configDefaults.exclude, "e2e/**"],
			environment: "jsdom",
			setupFiles: "./src/test-setup.ts",
		},
		base: mode === "production" ? "/static/" : "/",
		define: {
			"import.meta.env.VITE_API_HOST": JSON.stringify(apiHost),
		},
		server: {
			host: "::",
			port: 3000,
			proxy: {
				"/api": {
					target: "http://localhost:8080",
					changeOrigin: true,
				},
			},
		},
		plugins: [react(), mode === "development" && componentTagger()].filter(
			Boolean,
		),
		resolve: {
			alias: {
				"@": path.resolve(__dirname, "./src"),
			},
		},
	};
});
