import * as React from 'react';
import { useHistory, useLocation } from 'react-router-dom';
import {
  ActionGroup,
  Alert,
  Breadcrumb,
  BreadcrumbItem,
  Button,
  Form,
  FormGroup,
  FormSelect,
  FormSelectOption,
  PageSection,
  Switch,
  TextInput,
  Title,
} from '@patternfly/react-core';
import { createAPIClient } from '../utils/api';

const FIELD_TYPES = ['string', 'int', 'int64', 'bool', 'float', 'time'];

const FieldDefinitionCreatePage: React.FC = () => {
  const history = useHistory();
  const location = useLocation();
  const params = new URLSearchParams(location.search);
  const initialEntityDefId = params.get('entity_definition_id') || '';

  const [submitting, setSubmitting] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [entityDefinitionId, setEntityDefinitionId] = React.useState(initialEntityDefId);
  const [fieldName, setFieldName] = React.useState('');
  const [fieldType, setFieldType] = React.useState('string');
  const [isRequired, setIsRequired] = React.useState<boolean>(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    setError(null);

    const body: Record<string, unknown> = {
      entity_definition_id: entityDefinitionId,
      field_name: fieldName,
      field_type: fieldType,
      is_required: isRequired,
    };

    try {
      const api = createAPIClient();
      const created = await api.fieldDefinitions.create(body);
      history.push(`/trex-console/field-definitions/${created.id}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <>
      <PageSection variant="light">
        <Breadcrumb>
          <BreadcrumbItem
            onClick={() => history.push('/trex-console/field-definitions')}
            component="button"
          >
            Field Definitions
          </BreadcrumbItem>
          <BreadcrumbItem isActive>Create</BreadcrumbItem>
        </Breadcrumb>
        <Title headingLevel="h1">Create Field Definition</Title>
      </PageSection>
      <PageSection>
        {error && <Alert variant="danger" title="Error" isInline>{error}</Alert>}
        <Form onSubmit={handleSubmit} style={{ maxWidth: 600 }}>
          <FormGroup label="Entity Definition ID" fieldId="field-entity_definition_id" isRequired>
            <TextInput
              id="field-entity_definition_id"
              value={entityDefinitionId}
              onChange={(_e, val) => setEntityDefinitionId(val)}
              placeholder="Enter Entity Definition ID"
              isRequired
              isDisabled={!!initialEntityDefId}
            />
          </FormGroup>
          <FormGroup label="Field Name" fieldId="field-field_name" isRequired>
            <TextInput
              id="field-field_name"
              value={fieldName}
              onChange={(_e, val) => setFieldName(val)}
              placeholder="e.g. email, max_speed, created_by"
              isRequired
            />
          </FormGroup>
          <FormGroup label="Field Type" fieldId="field-field_type" isRequired>
            <FormSelect
              id="field-field_type"
              value={fieldType}
              onChange={(_e, val) => setFieldType(val)}
            >
              {FIELD_TYPES.map((t) => (
                <FormSelectOption key={t} value={t} label={t} />
              ))}
            </FormSelect>
          </FormGroup>
          <FormGroup label="Is Required" fieldId="field-is_required">
            <Switch
              id="field-is_required"
              isChecked={isRequired}
              onChange={(_e, val) => setIsRequired(val)}
            />
          </FormGroup>
          <ActionGroup>
            <Button type="submit" variant="primary" isLoading={submitting} isDisabled={submitting}>
              Create
            </Button>
            <Button
              variant="link"
              onClick={() => history.push('/trex-console/field-definitions')}
            >
              Cancel
            </Button>
          </ActionGroup>
        </Form>
      </PageSection>
    </>
  );
};

export default FieldDefinitionCreatePage;
