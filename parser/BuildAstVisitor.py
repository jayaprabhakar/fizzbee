
from antlr4 import *

from parser.FizzParser import FizzParser
from parser.FizzParserVisitor import FizzParserVisitor
import proto.fizz_ast_pb2 as ast

class BuildAstVisitor(FizzParserVisitor):
    def __init__(self, input_stream):
        super().__init__()
        self.input_stream = input_stream

    def aggregateResult(self, aggregate, nextResult):
        if nextResult is None:
            return aggregate
        return nextResult

    # Visit a parse tree produced by FizzParser#root.
    def visitRoot(self, ctx:FizzParser.RootContext):
        return self.visitFile_input(ctx.getChild(0))

    # Visit a parse tree produced by FizzParser#file_input.
    def visitFile_input(self, ctx:FizzParser.File_inputContext):
        print("\n\nvisitFile_input",ctx.__class__.__name__)
        print("visitFile_input",ctx.getText())
        print("visitFile_input",dir(ctx))
        print("visitFile_input children count",ctx.getChildCount())

        file = ast.File()

        for i, child in enumerate(ctx.getChildren()):
            print()
            print("visitFile_input child index",i,child.getText())
            if hasattr(child, 'toStringTree'):
                childProto = self.visit(child)
                if isinstance(childProto, ast.StateVars):
                    file.states.CopyFrom(childProto)
                elif isinstance(childProto, ast.Action):
                    file.actions.append(childProto)
                elif BuildAstVisitor.is_list_of_type(childProto):
                    file.invariants.extend(childProto)
                else:
                    print("visitFile_input childProto (unknown) type",childProto.__class__.__name__, dir(childProto), childProto)
                    raise Exception("visitFile_input childProto (unknown) type")
            elif hasattr(child, 'getSymbol'):
                if child.getSymbol().type == FizzParser.LINE_BREAK:
                    continue
                self.log_symbol(child)
            else:
                print("visitFile_input child (unknown) type",child.__class__.__name__, dir(child))
                raise Exception("visitFile_input child (unknown) type")
#         x = self.visitChildren(ctx)
#         print('visitFile_inputs children', x)
        print("file", file)
        return file

    def is_list_of_type(lst):
        # Check if all elements in the list are instances of ast.Invariant
        return all(isinstance(item, ast.Invariant) for item in lst)

    def visitInit_stmt(self, ctx:FizzParser.Init_stmtContext):
        init_str = self.input_stream.getText(ctx.start.start, ctx.stop.stop)
        py_str = BuildAstVisitor.transform_code(init_str, 1)
        return ast.StateVars(code=py_str)

    def transform_code(input_code, lines_to_skip=0):
        # Split the input code into lines
        lines = input_code.split('\n')

        # Remove the specified number of lines from the beginning
        del lines[:lines_to_skip]

        # Find the indentation of the second line
        indentation = len(lines[0]) - len(lines[0].lstrip())

        # Remove the same indentation from all subsequent lines
        transformed_code = '\n'.join(line[indentation:] if line.strip() else line for line in lines)

        return transformed_code


    # Visit a parse tree produced by FizzParser#visitActiondef.
    def visitActiondef(self, ctx:FizzParser.ActiondefContext):
        print("\n\nvisitActiondef",ctx.__class__.__name__)
        print("visitActiondef",ctx.getText())
        print("visitActiondef",dir(ctx))
        print("visitActiondef children count",ctx.getChildCount())
        print("visitActiondef full text\n",self.input_stream.getText(ctx.start.start, ctx.stop.stop))

        action = ast.Action()
        for i, child in enumerate(ctx.getChildren()):
            print()
            print("visitActiondef child index",i,child.getText())
            if hasattr(child, 'toStringTree'):
                if isinstance(child, FizzParser.NameContext):
                    action.name = child.getText()
                    continue

                self.log_childtree(child)
                childProto = self.visit(child)
                if isinstance(childProto, ast.Block):
                    action.block.CopyFrom(childProto)

                print("visitActiondef childProto",childProto)
            elif hasattr(child, 'getSymbol'):

                if (child.getSymbol().type == FizzParser.LINE_BREAK
                        or child.getSymbol().type == FizzParser.ACTION
                        or child.getSymbol().type == FizzParser.COLON
                ):
                    continue
                if child.getSymbol().type == FizzParser.ATOMIC:
                    action.flow = ast.Flow.FLOW_ATOMIC
                    continue
                if child.getSymbol().type == FizzParser.SERIAL:
                    action.flow = ast.Flow.FLOW_SERIAL
                    continue
                if child.getSymbol().type == FizzParser.ONEOF:
                    action.flow = ast.Flow.FLOW_ONEOF
                    continue
                if child.getSymbol().type == FizzParser.PARALLEL:
                    action.flow = ast.Flow.FLOW_PARALLEL
                    continue

                self.log_symbol(child)
            else:
                print("visitActiondef child (unknown) type",child.__class__.__name__, dir(child))
                raise Exception("visitActiondef child (unknown) type")

        if action.flow == ast.Flow.FLOW_UNKNOWN and action.block.flow != ast.Flow.FLOW_UNKNOWN:
            action.flow = action.block.flow
        elif action.flow != ast.Flow.FLOW_UNKNOWN and action.block.flow == ast.Flow.FLOW_UNKNOWN:
            action.block.flow = action.flow
        elif action.flow == ast.Flow.FLOW_UNKNOWN and action.block.flow == ast.Flow.FLOW_UNKNOWN:
            action.block.flow =  ast.Flow.FLOW_SERIAL
            action.flow = ast.Flow.FLOW_SERIAL

        print("action", action)
        return action

    # Visit a parse tree produced by FizzParser#expr_stmt.
    def visitExpr_stmt(self, ctx:FizzParser.Expr_stmtContext):
        py_str = self.input_stream.getText(ctx.start.start, ctx.stop.stop)
        print("visitExpr_stmt full text\n",py_str)
        py_str = BuildAstVisitor.transform_code(py_str)
        return ast.PyStmt(code=py_str)

    # Visit a parse tree produced by FizzParser#labelled_stmt.
    def visitLabelled_stmt(self, ctx:FizzParser.Labelled_stmtContext):
        print("\n\nvisitLabelled_stmt",ctx.__class__.__name__)
        print("visitLabelled_stmt\n",ctx.getText())
        block = None
        flow = ast.Flow.FLOW_UNKNOWN
        for i, child in enumerate(ctx.getChildren()):
            print()
            print("visitLabelled_stmt child index",i,child.getText())
            if hasattr(child, 'toStringTree'):
                self.log_childtree(child)
                childProto = self.visit(child)

                if isinstance(childProto, ast.Block):
                    block = childProto
                print("visitLabelled_stmt childProto",childProto)
            elif hasattr(child, 'getSymbol'):
                if (child.getSymbol().type == FizzParser.LINE_BREAK
                        or child.getSymbol().type == FizzParser.ACTION
                        or child.getSymbol().type == FizzParser.COLON
                        or child.getSymbol().type == FizzParser.INDENT
                ):
                    continue
                if child.getSymbol().type == FizzParser.ATOMIC:
                    flow = ast.Flow.FLOW_ATOMIC
                    continue
                if child.getSymbol().type == FizzParser.SERIAL:
                    flow = ast.Flow.FLOW_SERIAL
                    continue
                if child.getSymbol().type == FizzParser.ONEOF:
                    flow = ast.Flow.FLOW_ONEOF
                    continue
                if child.getSymbol().type == FizzParser.PARALLEL:
                    flow = ast.Flow.FLOW_PARALLEL
                    continue
                self.log_symbol(child)
            else:
                print("visitLabelled_stmt child (unknown) type",child.__class__.__name__, dir(child))
                raise Exception("visitLabelled_stmt child (unknown) type")

        if block is None:
            block = ast.Block()
        block.flow = flow
        print("visitLabelled_stmt block", block)
        return block

    # Visit a parse tree produced by FizzParser#suite.
    def visitSuite(self, ctx:FizzParser.SuiteContext):
        print("\n\nvisitSuite",ctx.__class__.__name__)
        print("visitSuite\n",ctx.getText())
        block = ast.Block()
        for i, child in enumerate(ctx.getChildren()):
            print()
            print("visitSuite child index",i,child.getText())
            if hasattr(child, 'toStringTree'):
                self.log_childtree(child)
                childProto = self.visit(child)
                if isinstance(childProto, ast.Statement):
                    block.stmts.append(childProto)
                    continue

                print("visitSuite childProto",childProto)
                raise Exception("visitSuite childProto (unknown) type", childProto.__class__.__name__, dir(childProto), childProto)
            elif hasattr(child, 'getSymbol'):

                if (child.getSymbol().type == FizzParser.LINE_BREAK
                        or child.getSymbol().type == FizzParser.ACTION
                        or child.getSymbol().type == FizzParser.COLON
                        or child.getSymbol().type == FizzParser.INDENT
                ):
                    continue

                self.log_symbol(child)
            else:
                print("visitSuite child (unknown) type",child.__class__.__name__, dir(child))
                raise Exception("visitSuite child (unknown) type")
        print("visitSuite block", block)
        if len(block.stmts) == 1 and block.stmts[0].block is not None:
            print("visitSuite block.stmts[0].block", block.stmts[0].block)
            return block.stmts[0].block
        return block

    # Visit a parse tree produced by FizzParser#stmt.
    def visitStmt(self, ctx:FizzParser.StmtContext):
        print("\n\nvisitStmt",ctx.__class__.__name__)
        if ctx.getChildCount() != 1:
            raise Exception("visitStmt child count != 1", ctx.getChildCount(), ctx.getText())
        childProto = self.visit(ctx.getChild(0))
        if childProto is None:
            return None
        if isinstance(childProto, ast.PyStmt):
            return ast.Statement(py_stmt=childProto)
        elif isinstance(childProto, ast.Block):
            return ast.Statement(block=childProto)
        elif isinstance(childProto, ast.StateVars):
            return childProto
        elif isinstance(childProto, ast.Action):
            return childProto
        elif isinstance(childProto, ast.Invariant):
            return childProto
        elif BuildAstVisitor.is_list_of_type(childProto):
            return childProto

        raise Exception("visitStmt childProto (unknown) type", childProto.__class__.__name__, dir(childProto), childProto)

    # Visit a parse tree produced by FizzParser#invariant_stmt.
    def visitInvariant_stmt(self, ctx:FizzParser.Invariant_stmtContext):
        print("\n\nvisitInvariant_stmt",ctx.__class__.__name__)
        print("visitInvariant_stmt\n",ctx.getText())
        invariant = ast.Invariant()
        for i, child in enumerate(ctx.getChildren()):
            print()
            print("visitInvariant_stmt child index",i,child.getText())
            if hasattr(child, 'toStringTree'):
                if isinstance(child, FizzParser.TestContext):
                    py_str = self.input_stream.getText(child.start.start, child.stop.stop)
                    print("visitExpr_stmt full text\n",py_str)
                    invariant.pyExpr = BuildAstVisitor.transform_code(py_str)
                    continue
                self.log_childtree(child)
                childProto = self.visit(child)
                print("visitInvariant_stmt childProto",childProto)
            elif hasattr(child, 'getSymbol'):
                if (child.getSymbol().type == FizzParser.LINE_BREAK
                        or child.getSymbol().type == FizzParser.COLON
                ):
                    continue
                if child.getSymbol().type == FizzParser.ALWAYS:
                    invariant.always = True
                    continue
                if child.getSymbol().type == FizzParser.EVENTUALLY:
                    invariant.eventually = True
                    continue
                self.log_symbol(child)
            else:
                print("visitInvariant_stmt child (unknown) type",child.__class__.__name__, dir(child))
                raise Exception("visitInvariant_stmt child (unknown) type")

        print("visitInvariant_stmt invariant", invariant)
        return invariant

    # Visit a parse tree produced by FizzParser#invariants_suite.
    def visitInvariants_suite(self, ctx:FizzParser.Invariants_suiteContext):
        invariants = []
        for i, child in enumerate(ctx.getChildren()):
            if hasattr(child, 'toStringTree'):
                childProto = self.visit(child)
                if isinstance(childProto, ast.Invariant):
                    invariants.append(childProto)
                else:
                    print("visitInvariants_suite childProto (unknown) type", childProto.__class__.__name__, dir(childProto), childProto)
                    raise Exception("visitInvariants_suite childProto (unknown) type")

        return invariants

    def log_symbol(self, child):
        print("log_symbol SymbolName",FizzParser.symbolicNames[child.getSymbol().type])
        print("log_symbol getSymbol",child.__class__.__name__,child.getSymbol(), dir(child))
        print("log_symbol symbol dir",dir(child.getSymbol()))
        print("log_symbol symbol type",child.getSymbol().type)

    def log_childtree(self, child):
        print("log_childtree child",child.__class__.__name__,child.getText())
        print("log_childtree child",dir(child))
        print("log_childtree child",child.getChildCount())
        print("log_childtree child",child.getRuleIndex())
        print("log_childtree child",child.getRuleContext())
        print("log_childtree child payloand",child.getPayload())
        print("log_childtree child full text\n",self.input_stream.getText(child.start.start, child.stop.stop))
        print("---")

