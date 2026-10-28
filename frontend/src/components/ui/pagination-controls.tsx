import {
  Pagination, PaginationContent, PaginationEllipsis, PaginationItem,
  PaginationLink, PaginationNext, PaginationPrevious,
} from "@/components/ui/pagination";

export interface PaginationControlsProps {
  currentPage: number;
  totalPages: number;
  onPageChange: (page: number) => void;
  className?: string;
}

export function PaginationControls({ currentPage, totalPages, onPageChange, className }: PaginationControlsProps) {
  if (!Number.isFinite(totalPages) || totalPages <= 1) return null;
  const pages = Math.trunc(totalPages);
  const page = Math.min(Math.max(1, Math.trunc(currentPage) || 1), pages);
  const canGoPrevious = page > 1;
  const canGoNext = page < pages;
  const showFirst = page > 2;
  const showLast = page < pages - 1;

  return (
    <Pagination className={className}>
      <PaginationContent>
        <PaginationItem><PaginationPrevious disabled={!canGoPrevious} onClick={() => canGoPrevious && onPageChange(page - 1)} className={!canGoPrevious ? 'opacity-50' : 'cursor-pointer'} /></PaginationItem>
        {showFirst && <>
          <PaginationItem><PaginationLink className="h-8 w-8 sm:h-10 sm:w-10" onClick={() => onPageChange(1)}>1</PaginationLink></PaginationItem>
          {page > 3 && <PaginationItem><PaginationEllipsis /></PaginationItem>}
        </>}
        {page > 1 && <PaginationItem><PaginationLink className="h-8 w-8 sm:h-10 sm:w-10" onClick={() => onPageChange(page - 1)}>{page - 1}</PaginationLink></PaginationItem>}
        <PaginationItem><PaginationLink className="h-8 w-8 sm:h-10 sm:w-10" isActive aria-label={`Page ${page}`} aria-current="page">{page}</PaginationLink></PaginationItem>
        {page < pages && <PaginationItem><PaginationLink className="h-8 w-8 sm:h-10 sm:w-10" onClick={() => onPageChange(page + 1)}>{page + 1}</PaginationLink></PaginationItem>}
        {showLast && <>
          {page < pages - 2 && <PaginationItem><PaginationEllipsis /></PaginationItem>}
          <PaginationItem><PaginationLink className="h-8 w-8 sm:h-10 sm:w-10" onClick={() => onPageChange(pages)}>{pages}</PaginationLink></PaginationItem>
        </>}
        <PaginationItem><PaginationNext disabled={!canGoNext} onClick={() => canGoNext && onPageChange(page + 1)} className={!canGoNext ? 'opacity-50' : 'cursor-pointer'} /></PaginationItem>
      </PaginationContent>
    </Pagination>
  );
}
