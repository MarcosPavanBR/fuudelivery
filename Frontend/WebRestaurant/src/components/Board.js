import React from "react";
import { DragDropContext } from "react-beautiful-dnd";
import Column from "./Column";

const Board = ({ columns, tasks, onDragEnd }) => {
  return (
    <DragDropContext onDragEnd={onDragEnd}>
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4 animate-fade-in">
        {columns.map((column) => (
          <Column
            key={column.id}
            column={column}
            tasks={tasks.filter((task) => task.column === column.id)}
            onDragEnd={onDragEnd}
          />
        ))}
      </div>
    </DragDropContext>
  );
};

export default Board;
