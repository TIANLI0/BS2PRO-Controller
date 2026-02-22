import * as React from "react";
import { cn } from "@/lib/utils";

const Card = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div
      ref={ref}
      className={cn("rounded-xl border border-gray-200 bg-white shadow-sm dark:border-gray-700 dark:bg-gray-800", className)}
      {...props}
    />
  )
);
Card.displayName = "Card";

export { Card };
