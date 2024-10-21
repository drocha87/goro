package main

const (
	COMPONENTS_BASEPATH = "components/"
	PAGES_BASEPATH      = "pages/"
	ASSETS_BASEPATH     = "assets/"
	DIST_BASEPATH       = "dist/"
	WEBCOMPONENT_PREFIX = "ecb-"

	// When set to true, enables discovery and registration of Web
	// Components that are defined using the 'is' attribute (e.g.,
	// is="WEBCOMPONENT_PREFIX..."). This is useful when extending
	// native HTML elements with custom behavior. If false, components
	// defined in this manner will be ignored. Only components prefixed
	// by WEBCOMPONENT_PREFIX will be added to the list of dependencies.
	// Worth noting that custom built-in elements has limited availability.
	ALLOW_IS_TAG_COMPONENTS = false

	VERBOSE = true
)
