# Goro - Static Website Generator

Goro is a simple yet flexible static website generator built in Go. It allows you to build websites by composing web components and pages from HTML templates. The generated site is organized into a `DIST_BASEPATH` directory with support for assets and customizable components.

## Features

- **Web Component Support**: Goro automatically registers and processes custom web components prefixed with `WEBCOMPONENT_PREFIX`. This allows for reusable, modular HTML structures across your site.
- **Asset Management**: Automatically copies and updates static assets from the `ASSETS_BASEPATH` folder to the `DIST_BASEPATH/assets` directory, ensuring your assets are always up to date.
- **Recursive and Cyclic Detection**: Ensures that your components are not recursively or cyclically dependent, avoiding rendering issues.
- **Template-based Pages**: Build your website pages from HTML templates stored in the `PAGES_BASEPATH` directory.

## Project Structure

- `COMPONENTS_BASEPATH/`: Contains reusable HTML components that can be injected into pages.
- `PAGES_BASEPATH/`: HTML templates for each page on the site.
- `ASSETS_BASEPATH/`: Static files (e.g., CSS, JS, images) that will be copied to the `DIST_BASEPATH/assets/` directory.
- `DIST_BASEPATH/`: The output folder containing the fully generated site ready to be deployed.

## How It Works

1. **Component Registration**: The `loadComponent()` function scans the `COMPONENTS_BASEPATH/` directory for HTML files. Components with the `WEBCOMPONENT_PREFIX` prefix are registered and can be reused across different pages.
2. **Page Compilation**: Each HTML template in the `PAGES_BASEPATH/` folder is parsed, compiled, and output into the `DIST_BASEPATH/` directory.
3. **Asset Copying**: Assets in the `ASSETS_BASEPATH/` folder are copied to the `DIST_BASEPATH/assets` directory. If an asset is already present and up to date, it will be skipped.
4. **Component Population**: Components are populated by analyzing their dependencies and checking for cyclic or recursive references before rendering.

## Installation

1. Clone this repository:
   ```bash
   git clone https://github.com/yourusername/goro.git
   ```
2. Install dependencies:
   ```bash
   go get
   ```
3. Build the project:
   ```bash
   go build
   ```
4. Run Goro to generate your website:
   ```
   ./goro
   ```

## Configuration

**Web Component Prefix**

The default prefix for custom web components is `WEBCOMPONENT_PREFIX`. You can modify this in the source code by adjusting the WEBCOMPONENT_PREFIX constant in `config.go`.

**Allow `is` Attribute Components**

Goro supports custom web components that extend native HTML elements using the `is` attribute. This feature is disabled by default due to limited browser support and potential performance considerations. To enable this feature, set the `ALLOW_IS_TAG_COMPONENTS` constant to `true` in the source code.

For example, enabling this would allow you to register and use components like:
```html
<button is="wc-custom-button">Click Me</button>
```

**Note**: Only components with the `WEBCOMPONENT_PREFIX` (e.g., `wc-`) will be processed when this feature is enabled.

To enable this feature:
1. Open the source file and locate the `ALLOW_IS_TAG_COMPONENTS` constant.
2. Set it to `true`:
```go
const ALLOW_IS_TAG_COMPONENTS = true
```

Keep in mind that support for the is attribute may vary across browsers, so it's recommended to test your site in different environments when using this feature.

## TODO

- [ ] **Command-line Flags & Configuration File Support**  
  Implement support for configuring `goro` through command-line flags or an external configuration file. This will allow users to specify important settings (like paths and flags) without needing to modify the source code.

- [ ] **HTML Fragment Support**  
  Add support for accepting partial HTML files (fragments) that can be inserted directly into pages without parsing or modification. This feature will make it easier to include raw HTML in specific parts of a page.

- [ ] **Watch Mode for Automatic Rebuilds**  
  Enable a watch mode that detects changes in the `COMPONENTS_BASEPATH`, `PAGES_BASEPATH`, and `ASSETS_BASEPATH`. When files are modified, `goro` will automatically trigger a rebuild, streamlining the development process.

- [ ] **`goro init` Command**  
  Introduce a `goro init` command to initialize a new project folder with default configurations. This command will generate a basic directory structure and a configuration file, helping users quickly set up their projects.

- [ ] **Individual Page Compilation**  
  Implement functionality to compile individual pages on demand instead of regenerating the entire site. This will speed up the build process when making small changes to a single page.

- [ ] **Unused Components Report**  
  Add a feature that reports any components defined in the `COMPONENTS_BASEPATH/` directory that are not used anywhere in the project. This will help identify and clean up unused components.

- [ ] **Configurable Log Level**  
  Allow users to set the desired log level (e.g., info, warning, error) to reduce verbose output in the terminal during build processes. This will improve the user experience by limiting log noise during routine operations.
