import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

export interface PageSizeSelectorProps {
  value: number;
  onChange: (value: number) => void;
  options?: number[];
  label?: string;
  className?: string;
}

export function PageSizeSelector({
  value,
  onChange,
  options = [10, 25, 50, 100],
  label = "Items per page",
  className,
}: PageSizeSelectorProps) {
  const allOptions = [0, ...options.filter(o => o > 0)];

  return (
    <div className={`flex items-center gap-2 ${className || ''}`}>
      <span className="text-sm text-muted-foreground whitespace-nowrap">
        {label}:
      </span>
      <Select
        value={value.toString()}
        onValueChange={(val) => onChange(parseInt(val, 10))}
      >
        <SelectTrigger className="w-[100px]">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          {allOptions.map((option) => (
            <SelectItem key={option} value={option.toString()}>
              {option === 0 ? 'All' : option}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  );
}
