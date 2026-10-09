import { cva } from "class-variance-authority";

export const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap rounded-lg text-sm font-medium transition-colors disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg:not([class*='size-'])]:size-4 shrink-0 [&_svg]:shrink-0 outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2",
  {
    variants: {
      variant: {
        default:
          "bg-primary text-primary-foreground shadow-2xs hover:bg-primary/90",
        destructive:
          "bg-destructive text-destructive-foreground shadow-2xs hover:bg-destructive/90",
        outline:
          "border border-border/80 bg-background shadow-2xs hover:bg-muted hover:text-foreground",
        secondary:
          "bg-secondary text-secondary-foreground shadow-2xs hover:bg-secondary/80",
        ghost:
          "hover:bg-muted hover:text-foreground",
        link: "text-primary underline-offset-4 hover:underline",
        soft: "bg-muted text-foreground hover:bg-muted/80 shadow-none",
        solid: "bg-primary text-primary-foreground shadow-2xs hover:bg-primary/90",
      },
      size: {
        default: "h-9 px-3.5 py-2 has-[>svg]:px-3",
        sm: "h-8 rounded-md gap-1.5 px-2.5 text-xs has-[>svg]:px-2",
        lg: "h-10 rounded-lg px-6 has-[>svg]:px-4 text-base",
        icon: "size-9",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  },
);
