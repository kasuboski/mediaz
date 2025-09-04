import { defineConfig } from "vite";
import react from "@vitejs/plugin-react-swc";
import path from "node:path";
import { componentTagger } from "lovable-tagger";

export default defineConfig(({ mode }) => {
	const apiHost = mode === "production" ? "" : "http://localhost:8080";

	return {
		base: mode === "production" ? "/static/" : "/",
		define: {
			"import.meta.env.VITE_API_HOST": JSON.stringify(apiHost),
		},
		server: {
			host: "::",
			port: 8080,
		},
		plugins: [react(), mode === "development" && componentTagger()].filter(Boolean),
		resolve: {
			alias: {
				"@": path.resolve(__dirname, "./src"),
			},
		},
	};
});