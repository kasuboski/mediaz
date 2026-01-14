import { useState, useEffect } from "react";
import { Clock } from "lucide-react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export interface TimeRangeSelectorProps {
  days: number;
  onChange: (days: number) => void;
  className?: string;
}

const TIME_RANGE_OPTIONS = [
  { label: "Today", days: 0 },
  { label: "24H", days: 1 },
  { label: "7D", days: 7 },
  { label: "30D", days: 30 },
  { label: "All", days: 365 },
] as const;

export function TimeRangeSelector({
  days,
  onChange,
  className,
}: TimeRangeSelectorProps) {
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  const handleSelect = (days: number) => {
    onChange(days);
  };

  if (!mounted) {
    return null;
  }

  return (
    <div className={cn("flex items-center gap-2", className)}>
      <Clock className="h-4 w-4 text-muted-foreground" />
      <div className="inline-flex rounded-md border border-border p-1 bg-background">
        {TIME_RANGE_OPTIONS.map((option) => (
          <Button
            key={option.days}
            variant={days === option.days ? "default" : "ghost"}
            size="sm"
            onClick={() => handleSelect(option.days)}
            className="h-8 px-3 text-xs font-medium"
          >
            {option.label}
          </Button>
        ))}
      </div>
    </div>
  );
}
