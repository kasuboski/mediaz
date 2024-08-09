# mediaz

## Configuration Loading with Viper

The application configuration is managed using Viper, which allows for flexible and hierarchical configuration sourcing. Here's how the configuration is loaded:

1. **Configuration File (Optional)**:
   - If a configuration file is provided (e.g., via the `--config` flag), Viper will attempt to read the file and load its contents. The file can be in formats such as YAML, JSON, or TOML.

2. **Environment Variables**:
   - Environment variables are automatically loaded and can override values set in the configuration file. Environment variables are prefixed with `MEDIAZ` to avoid conflicts and are transformed to match the configuration keys using the following rules:
     - Periods (`.`) and hyphens (`-`) in keys are replaced with underscores (`_`).
     - For example, the environment variable `MEDIAZ_TMDB_APIKEY` maps to the configuration key `tmdb.apikey`.

3. **Defaults**:
   - Default values are set programmatically for key configuration settings. These defaults ensure that the application can run even if certain configuration values are not provided via a file or environment variables.
   - The default configuration includes:
     - `tmdb.scheme`: Defaults to `"https"`.
     - `tmdb.host`: Defaults to `"api.themoviedb.org"`.
     - `tmdb.apikey`: Defaults to an empty string, ready to be overridden by the corresponding environment variable.

4. **Priority Order**:
   - Viper follows a priority order when loading configuration values:
     1. **Command-line flags** (highest priority).
     2. **Environment variables**.
     3. **Configuration file**.
     4. **Defaults** (lowest priority).

This hierarchy ensures that the application configuration is both flexible and easily overridden based on the deployment environment.
