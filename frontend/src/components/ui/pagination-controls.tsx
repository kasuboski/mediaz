import {
  Pagination,
  PaginationContent,
  PaginationEllipsis,
  PaginationItem,
  PaginationLink,
  PaginationNext,
  PaginationPrevious,
} from "@/components/ui/pagination";

export interface PaginationControlsProps {
  currentPage: number;
  totalPages: number;
  onPageChange: (page: number) => void;
  className?: string;
}

export function PaginationControls({
  currentPage,
  totalPages,
  onPageChange,
  className,
}: PaginationControlsProps) {
  if (totalPages <= 1) {
    return null;
  }

  const canGoPrevious = currentPage > 1;
  const canGoNext = currentPage < totalPages;
  const showFirst = currentPage > 2;
  const showLast = currentPage < totalPages - 1;

  return (
    <Pagination className={className}>
      <PaginationContent>
        <PaginationItem>
          <PaginationPrevious
            onClick={() => canGoPrevious && onPageChange(currentPage - 1)}
            className={!canGoPrevious ? 'pointer-events-none opacity-50' : 'cursor-pointer'}
          />
        </PaginationItem>

        {showFirst && (
          <>
            <PaginationItem>
              <PaginationLink onClick={() => onPageChange(1)} className="cursor-pointer">
                1
              </PaginationLink>
            </PaginationItem>
            {currentPage > 3 && (
              <PaginationItem>
                <PaginationEllipsis />
              </PaginationItem>
            )}
          </>
        )}

        {currentPage > 1 && (
          <PaginationItem>
            <PaginationLink onClick={() => onPageChange(currentPage - 1)} className="cursor-pointer">
              {currentPage - 1}
            </PaginationLink>
          </PaginationItem>
        )}

        <PaginationItem>
          <PaginationLink isActive className="cursor-pointer">
            {currentPage}
          </PaginationLink>
        </PaginationItem>

        {currentPage < totalPages && (
          <PaginationItem>
            <PaginationLink onClick={() => onPageChange(currentPage + 1)} className="cursor-pointer">
              {currentPage + 1}
            </PaginationLink>
          </PaginationItem>
        )}

        {showLast && (
          <>
            {currentPage < totalPages - 2 && (
              <PaginationItem>
                <PaginationEllipsis />
              </PaginationItem>
            )}
            <PaginationItem>
              <PaginationLink onClick={() => onPageChange(totalPages)} className="cursor-pointer">
                {totalPages}
              </PaginationLink>
            </PaginationItem>
          </>
        )}

        <PaginationItem>
          <PaginationNext
            onClick={() => canGoNext && onPageChange(currentPage + 1)}
            className={!canGoNext ? 'pointer-events-none opacity-50' : 'cursor-pointer'}
          />
        </PaginationItem>
      </PaginationContent>
    </Pagination>
  );
}
