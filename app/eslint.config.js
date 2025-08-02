import js from "@eslint/js";
import globals from "globals";

export default [
    js.configs.recommended,
    { ignores: ["vite.config.js", "dist/*"] },
    {
        files: ["**/*.js"],
        languageOptions: {
            globals: globals.browser
        }
    },
];
