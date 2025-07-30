# Mediaz Design System Documentation

## Overview

The Mediaz design system is a comprehensive design language for a media discovery and request application. It features a dark theme with blue accents inspired by Overseerr, designed to provide an elegant and modern interface for browsing movies and TV shows.

## Visual Identity

### Brand Colors

The design system is built around a sophisticated dark theme with vibrant blue accents:

- **Brand Blue**: `hsl(213, 94%, 68%)` - A bright, engaging blue used for primary actions and accents
- **Deep Blue Variant**: `hsl(213, 84%, 58%)` - A slightly darker variant used in gradients

### Theme Philosophy

- **Dark-first approach**: All components are designed with a dark background as the primary theme
- **High contrast**: Ensures excellent readability with carefully chosen text colors
- **Blue accent system**: Uses blue as the primary accent color for interactive elements and branding
- **Gradient emphasis**: Leverages gradients to create depth and visual interest

## Color System

### Core Colors

#### Background Colors
- **Primary Background**: `hsl(220, 13%, 9%)` - Main app background
- **Card Background**: `hsl(220, 13%, 11%)` - Card and panel backgrounds
- **Secondary Background**: `hsl(220, 13%, 15%)` - Secondary surfaces
- **Input Background**: `hsl(220, 13%, 15%)` - Form input backgrounds

#### Text Colors
- **Primary Text**: `hsl(220, 9%, 95%)` - Main text color
- **Muted Text**: `hsl(220, 9%, 65%)` - Secondary text and descriptions
- **Primary Foreground**: `hsl(220, 13%, 9%)` - Text on blue backgrounds

#### Interactive Colors
- **Primary**: `hsl(213, 94%, 68%)` - Primary buttons, links, and active states
- **Destructive**: `hsl(0, 84%, 60%)` - Error states and dangerous actions
- **Border**: `hsl(220, 13%, 20%)` - Border color for components
- **Ring**: `hsl(213, 94%, 68%)` - Focus ring color

#### Sidebar-Specific Colors
- **Sidebar Background**: `hsl(220, 13%, 8%)` - Slightly darker than main background
- **Sidebar Foreground**: `hsl(220, 9%, 85%)` - Sidebar text
- **Sidebar Accent**: `hsl(220, 13%, 12%)` - Sidebar hover and active states
- **Sidebar Border**: `hsl(220, 13%, 18%)` - Sidebar internal borders

### Usage Guidelines

#### Color Accessibility
- All text color combinations meet WCAG AA standards for contrast
- Focus states use high-contrast blue for keyboard navigation
- Error states use red with sufficient contrast against dark backgrounds

#### Color Hierarchy
1. **Primary Blue**: Used sparingly for key actions and brand elements
2. **White/Light Gray**: Primary text and important information
3. **Medium Gray**: Secondary text and supporting information
4. **Dark Gray**: Backgrounds and subtle elements

## Typography

### Font Features
- **OpenType Features**: `"rlig" 1, "calt" 1` - Enables contextual alternates and ligatures
- **System Font Stack**: Relies on system fonts for optimal performance and native feel

### Text Utilities
- **Line Clamping**: Built-in utilities for 1, 2, and 3-line text truncation
- **Font Weight Scale**: Medium weight (500) used for emphasized text and headings
- **Letter Spacing**: Tracking adjustments for uppercase labels and small text

## Layout System

### Grid and Spacing
- **Container**: Centered with 2rem padding, max-width of 1400px on 2xl screens
- **Responsive Design**: Mobile-first approach with standard breakpoints
- **Flexbox-based**: Primary layout mechanism for consistent alignment

### Layout Patterns

#### Application Shell
```
┌─────────────────────────────────────┐
│ Sidebar (64px wide)    │ Header     │
│ ┌─────────────────────┐│ ┌─────────┐│
│ │ Logo + Navigation   ││ │ Search  ││
│ │                     ││ └─────────┘│
│ │ • Discover          ││            │
│ │ • Movies            ││ Main       │
│ │ • TV Shows          ││ Content    │
│ │                     ││ Area       │
│ │ Manage              ││            │
│ │ • Library           ││            │
│ │ • Settings          ││            │
│ └─────────────────────┘│            │
└─────────────────────────────────────┘
```

#### Content Grid
- **Media Grid**: Responsive grid for movie/TV show posters
- **Aspect Ratios**: 2:3 ratio for media posters (movie poster standard)
- **Card Spacing**: Consistent gaps between grid items

## Component Architecture

### Design Principles
1. **Consistency**: All components follow the same design patterns
2. **Reusability**: Components are designed to work in multiple contexts
3. **Accessibility**: Built with keyboard navigation and screen readers in mind
4. **Performance**: Optimized for smooth animations and interactions

### Core Components

#### Cards
- **Background**: Gradient background using `bg-gradient-card`
- **Borders**: Subtle borders with `border-border/50`
- **Shadows**: Layered shadow system (card, card-hover, modal)
- **Hover Effects**: Scale and shadow transitions on hover
- **Content Structure**: Consistent padding and text hierarchy

#### Buttons
- **Primary**: Blue gradient background with white text
- **Secondary**: Muted background with light text
- **Ghost**: Transparent with hover background
- **Destructive**: Red background for dangerous actions

#### Navigation
- **Sidebar**: Fixed-width sidebar with brand header and grouped navigation
- **Active States**: Blue accent color for current page
- **Hover States**: Subtle background changes on hover
- **Icons**: Consistent icon usage with lucide-react

#### Forms
- **Inputs**: Dark backgrounds with light borders
- **Focus States**: Blue ring on focus
- **Placeholder Text**: Muted foreground color
- **Labels**: Medium font weight for form labels

## Animation and Transitions

### Transition System
- **Smooth Transitions**: `all 0.3s cubic-bezier(0.4, 0, 0.2, 1)` for general animations
- **Fast Transitions**: `all 0.15s cubic-bezier(0.4, 0, 0.2, 1)` for quick interactions
- **Hover Effects**: Transform and shadow changes on hover
- **Loading States**: Spinner animations for async operations

### Motion Guidelines
1. **Subtle Movement**: Gentle hover effects that don't distract
2. **Consistent Timing**: Standard easing curves across all animations
3. **Performance**: GPU-accelerated transforms for smooth animations
4. **Reduced Motion**: Should respect user's reduced motion preferences

## Gradients and Effects

### Gradient System
- **Primary Gradient**: `linear-gradient(135deg, hsl(213 94% 68%) 0%, hsl(213 84% 58%) 100%)`
- **Card Gradient**: `linear-gradient(145deg, hsl(220 13% 11%) 0%, hsl(220 13% 13%) 100%)`
- **Hero Gradient**: `linear-gradient(180deg, hsl(220 13% 9% / 0.4) 0%, hsl(220 13% 9%) 100%)`

### Shadow System
- **Card Shadow**: `0 4px 12px -4px hsl(220 13% 5% / 0.4)`
- **Card Hover Shadow**: `0 8px 25px -8px hsl(213 94% 68% / 0.15)`
- **Modal Shadow**: `0 25px 50px -12px hsl(220 13% 5% / 0.8)`

## Responsive Design

### Breakpoint Strategy
- **Mobile-first**: Base styles for mobile, enhanced for larger screens
- **Container Queries**: Where appropriate for component-level responsiveness
- **Flexible Layouts**: Use of flexbox and grid for adaptive layouts

### Responsive Patterns
1. **Sidebar**: Collapsible on mobile with hamburger trigger
2. **Grid Layouts**: Responsive columns based on screen size
3. **Typography**: Responsive font sizes using Tailwind's responsive utilities

## Content Guidelines

### Media Display
- **Poster Images**: Always use 2:3 aspect ratio for consistency
- **Image Loading**: Implement lazy loading for performance
- **Fallbacks**: Placeholder images for missing poster artwork
- **Image Sources**: TMDB image URLs with appropriate sizing

### Text Content
- **Truncation**: Use line-clamp utilities for consistent text overflow handling
- **Hierarchy**: Clear visual hierarchy with font weights and colors
- **Spacing**: Consistent spacing between text elements

## Accessibility Standards

### WCAG Compliance
- **Color Contrast**: All text meets WCAG AA standards (4.5:1 ratio minimum)
- **Focus Indicators**: Visible focus states for keyboard navigation
- **Screen Reader Support**: Proper semantic HTML and ARIA labels
- **Keyboard Navigation**: All interactive elements accessible via keyboard

### Implementation Requirements
1. **Focus Management**: Logical tab order throughout the application
2. **Alt Text**: Descriptive alt text for all images
3. **Form Labels**: Proper labeling for all form inputs
4. **Role Attributes**: Appropriate ARIA roles for custom components

## Implementation Guidelines

### CSS Custom Properties
Use the defined CSS custom properties for colors:
```css
/* Example usage */
.my-component {
  background-color: hsl(var(--card));
  color: hsl(var(--card-foreground));
  border: 1px solid hsl(var(--border));
}
```

### Tailwind CSS Integration
The system is fully integrated with Tailwind CSS:
```jsx
// Example component usage

    Primary Action

```

### Component Development
1. **Use Design Tokens**: Always reference CSS custom properties
2. **Follow Naming Conventions**: Use semantic naming for components and classes  
3. **Maintain Consistency**: Stick to established patterns for spacing, colors, and typography
4. **Test Accessibility**: Verify keyboard navigation and screen reader compatibility

## Quality Assurance

### Design Review Checklist
- [ ] Colors match the defined palette
- [ ] Proper contrast ratios maintained
- [ ] Consistent spacing and typography
- [ ] Responsive behavior across screen sizes
- [ ] Hover and focus states implemented
- [ ] Loading states for async operations
- [ ] Error states properly styled
- [ ] Accessibility requirements met

### Performance Considerations
- **Optimize Images**: Use appropriate image sizes and formats
- **Minimize Repaints**: Use transform properties for animations
- **Efficient Selectors**: Leverage Tailwind's utility classes
- **Bundle Size**: Monitor CSS bundle size and optimize when necessary

This design system provides a comprehensive foundation for building consistent, accessible, and visually appealing interfaces within the Mediaz application ecosystem.