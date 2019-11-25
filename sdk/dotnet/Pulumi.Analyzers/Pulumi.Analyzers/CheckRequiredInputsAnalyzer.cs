// Copyright 2016-2019, Pulumi Corporation

using System.Collections.Generic;
using System.Collections.Immutable;
using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.Diagnostics;
using Microsoft.CodeAnalysis.Operations;

namespace Pulumi.Analyzers
{
    [DiagnosticAnalyzer(LanguageNames.CSharp, LanguageNames.VisualBasic)]
    public class CheckRequiredInputsAnalyzer : DiagnosticAnalyzer
    {
        public const string DiagnosticId = "PU0001";

        private static DiagnosticDescriptor Rule = new DiagnosticDescriptor(
            DiagnosticId,
            new LocalizableResourceString(nameof(Resources.AnalyzerTitle), Resources.ResourceManager, typeof(Resources)),
            new LocalizableResourceString(nameof(Resources.AnalyzerMessageFormat), Resources.ResourceManager, typeof(Resources)),
            "Naming", DiagnosticSeverity.Warning, isEnabledByDefault: true);

        public override ImmutableArray<DiagnosticDescriptor> SupportedDiagnostics { get { return ImmutableArray.Create(Rule); } }

        public override void Initialize(AnalysisContext context)
        {
            context.RegisterCompilationStartAction(ctx =>
            {
                if (!Analyzer.TryCreate(ctx.Compilation, out var analyzer))
                    return;

                ctx.RegisterOperationBlockAction(analyzer.AnalyzeBlock);
            });
        }

        private class Analyzer
        {
            private readonly INamedTypeSymbol _inputArgsType;
            private readonly INamedTypeSymbol _inputAttributeType;

            private readonly Dictionary<ILocalSymbol, (ITypeSymbol type, HashSet<IPropertySymbol> assignedProperties)> _localToInfoMap
                = new Dictionary<ILocalSymbol, (ITypeSymbol type, HashSet<IPropertySymbol> assignedProperties)>();

            private readonly Dictionary<ITypeSymbol, IImmutableSet<IPropertySymbol>> _typeToRequiredPropertiesMap
                = new Dictionary<ITypeSymbol, IImmutableSet<IPropertySymbol>>();

            public Analyzer(INamedTypeSymbol inputArgsType, INamedTypeSymbol inputAttributeType)
            {
                _inputArgsType = inputArgsType;
                _inputAttributeType = inputAttributeType;
            }

            internal static bool TryCreate(Compilation compilation, out Analyzer analyzer)
            {
                var inputArgsType = compilation.GetTypeByMetadataName("Pulumi.InputArgs");
                var inputAttributeType = compilation.GetTypeByMetadataName("Pulumi.Serialization.InputAttribute");

                if (inputArgsType == null || inputAttributeType == null)
                {
                    analyzer = null;
                    return false;
                }

                analyzer = new Analyzer(inputArgsType, inputAttributeType);
                return true;
            }

            public void AnalyzeBlock(OperationBlockAnalysisContext context)
            {
                // Walk each top-level operation block looking for all instantiations of pulumi arg types.
                var cancellationToken = context.CancellationToken;
                foreach (var block in context.OperationBlocks)
                {
                    cancellationToken.ThrowIfCancellationRequested();

                    _localToInfoMap.Clear();
                    AnalyzeOperation(context, block);

                    // At the end of each top level block, report any locals that were initialized
                    // to a pulumi arg type, but which did not have all required properties assigned
                    // to them.
                    foreach (var kvp in _localToInfoMap)
                    {
                        var local = kvp.Key;
                        var (type, assignedProperties) = kvp.Value;

                        EnsureAllRequiredProperties(
                            context, type, assignedProperties,
                            local.DeclaringSyntaxReferences[0].GetSyntax(cancellationToken));
                    }
                }
            }

            private void AnalyzeOperation(OperationBlockAnalysisContext context, IOperation operation)
            {
                context.CancellationToken.ThrowIfCancellationRequested();

                // Look for `new SomePulumiArgType`
                if (operation is IObjectCreationOperation objectCreation)
                {
                    var objectType = objectCreation.Type;
                    if (IsInputCreation(objectType))
                    {
                        // See which properties were provided directly at construction time. i.e.
                        // `new SomePulumiArgType { Name = ... }`
                        var assignedProperties = GetAssignedProperties(objectCreation);

                        // If we're assigning to a local, then we allow code later on to assign 
                        // the other required properties.
                        if (operation.Parent is IVariableInitializerOperation variableInitializer &&
                            variableInitializer.Parent is IVariableDeclaratorOperation variableDeclarator)
                        {
                            _localToInfoMap[variableDeclarator.Symbol] = (objectType, assignedProperties);
                        }
                        else
                        {
                            // Otherwise, we're just new'ing up an InputArgs type, but not assigning it
                            // anywhere.  We need all the required properties to be assigned.
                            EnsureAllRequiredProperties(
                                context, objectType, assignedProperties, objectCreation.Syntax);
                        }
                    }
                }

                // Look for property assignments to a local variable we're tracking.  i.e.
                //
                //      var v = new SomePulumiArgType();
                //      v.Name = ...
                if (operation is IPropertyReferenceOperation propertyReference &&
                    propertyReference.Instance is ILocalReferenceOperation localReference &&
                    _localToInfoMap.ContainsKey(localReference.Local) &&
                    propertyReference.Parent is ISimpleAssignmentOperation simpleAssignment &&
                    simpleAssignment.Target == propertyReference)
                {
                    _localToInfoMap[localReference.Local].assignedProperties.Add(propertyReference.Property);
                }

                // Recurse and analyze the rest of the block.
                foreach (var child in operation.Children)
                {
                    AnalyzeOperation(context, child);
                }
            }

            private void EnsureAllRequiredProperties(
                OperationBlockAnalysisContext context, ITypeSymbol objectType,
                HashSet<IPropertySymbol> assignedProperties, SyntaxNode syntax)
            {
                var requiredProperties = GetRequiredProperties(objectType);
                foreach (var missing in requiredProperties.Except(assignedProperties))
                {
                    context.ReportDiagnostic(Diagnostic.Create(
                        Rule, syntax.GetFirstToken().GetLocation(), missing.Name));
                }
            }

            private bool IsInputCreation(ITypeSymbol type)
            {
                // We're creating some input arg if this type derives from `Pulumi.InputArgs`
                for (var current = type?.OriginalDefinition; current != null; current = current.BaseType)
                {
                    if (current.Equals(_inputArgsType))
                    {
                        return true;
                    }
                }

                return false;
            }

            private HashSet<IPropertySymbol> GetAssignedProperties(IObjectCreationOperation objectCreation)
            {
                var result = new HashSet<IPropertySymbol>();

                if (objectCreation.Initializer != null)
                {
                    foreach (var initializer in objectCreation.Initializer.Initializers)
                    {
                        if (initializer is ISimpleAssignmentOperation assignmentOperation &&
                            assignmentOperation.Target is IPropertyReferenceOperation propertyReference)
                        {
                            result.Add(propertyReference.Property);
                        }
                    }
                }

                return result;
            }

            private IImmutableSet<IPropertySymbol> GetRequiredProperties(ITypeSymbol type)
            {
                if (!_typeToRequiredPropertiesMap.TryGetValue(type, out var requiredProperties))
                {
                    requiredProperties = ComputeRequiredProperties(type);
                    _typeToRequiredPropertiesMap[type] = requiredProperties;
                }

                return requiredProperties;
            }

            private IImmutableSet<IPropertySymbol> ComputeRequiredProperties(ITypeSymbol type)
            {
                var result = ImmutableHashSet.CreateBuilder<IPropertySymbol>();

                // Walk all the properties of this args type looking for those that have a
                // `[Input("name", required: true)]` arg.
                foreach (var member in type.GetMembers())
                {
                    if (member is IPropertySymbol property)
                    {
                        var attributes = property.GetAttributes();
                        foreach (var attribute in attributes)
                        {
                            if (_inputAttributeType.Equals(attribute.AttributeClass))
                            {
                                if (attribute.ConstructorArguments.Length >= 2 &&
                                    attribute.ConstructorArguments[1].Value is true)
                                {
                                    result.Add(property);
                                }
                            }
                        }
                    }
                }

                return result.ToImmutable();
            }
        }
    }
}
